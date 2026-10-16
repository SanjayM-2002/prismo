package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

// dialect turns migration steps into SQL and reads tables back from one
// kind of database.
type dialect interface {
	SQL(step migrate.Step) (string, error)
	Tables(ctx context.Context, db *sql.DB) ([]migrate.Table, error)
}

// push makes the database match the schema. Steps that can lose data are
// only applied with --accept-data-loss; without it, nothing is applied.
func push(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("db push", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("schema", "schema.prismo", "path to the schema file")
	acceptDataLoss := flags.Bool("accept-data-loss", false, "apply changes that can lose data")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	s, err := loadSchema(*path)
	if err != nil {
		return err
	}
	p, err := providerFor(s.Datasource.Provider)
	if err != nil {
		return err
	}
	url, err := datasourceURL(s.Datasource)
	if err != nil {
		return err
	}

	db, err := p.open(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()

	have, err := p.dialect.Tables(ctx, db)
	if err != nil {
		return fmt.Errorf("read database: %w", err)
	}
	steps, err := migrate.Diff(migrate.Tables(s), have)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		fmt.Fprintln(stdout, "The database is already in sync with the schema.")
		return nil
	}

	// Render every statement first, so a rendering error can't leave the
	// database half-changed.
	queries := make([]string, len(steps))
	var warnings []string
	fmt.Fprintln(stdout, "Changes:")
	for i, step := range steps {
		fmt.Fprintf(stdout, "  %s\n", step.Describe())
		if queries[i], err = p.dialect.SQL(step); err != nil {
			return fmt.Errorf("%s: %w", step.Describe(), err)
		}
		if w := step.Warning(); w != "" {
			warnings = append(warnings, w)
		}
	}

	if len(warnings) > 0 && !*acceptDataLoss {
		fmt.Fprintln(stdout, "\nThese changes can lose data:")
		for _, w := range warnings {
			fmt.Fprintf(stdout, "  %s\n", w)
		}
		return errors.New("nothing was applied; run again with --accept-data-loss to apply these changes")
	}

	for i, step := range steps {
		if _, err := db.ExecContext(ctx, queries[i]); err != nil {
			return fmt.Errorf("%s: %w%s", step.Describe(), err, appliedNote(steps[:i]))
		}
	}
	fmt.Fprintf(stdout, "Applied %d %s.\n", len(steps), plural(len(steps), "change", "changes"))
	return nil
}

// appliedNote lists the steps that ran before a failure. MySQL can't roll
// back schema changes, so the user needs to know what already happened.
func appliedNote(applied []migrate.Step) string {
	if len(applied) == 0 {
		return " (nothing was applied)"
	}
	names := make([]string, len(applied))
	for i, step := range applied {
		names[i] = step.Describe()
	}
	return " (already applied: " + strings.Join(names, ", ") + ")"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// loadSchema reads, parses and validates the schema file at path.
func loadSchema(path string) (*schema.Schema, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	s, err := schema.Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := schema.Validate(s); err != nil {
		return nil, fmt.Errorf("%s: invalid schema:\n%w", path, err)
	}
	return s, nil
}

// datasourceURL returns the connection URL, reading it from the environment
// when the schema uses env("NAME").
func datasourceURL(ds *schema.Datasource) (string, error) {
	if ds.URLEnv == "" {
		return ds.URL, nil
	}
	url := os.Getenv(ds.URLEnv)
	if url == "" {
		return "", fmt.Errorf("environment variable %s is not set", ds.URLEnv)
	}
	return url, nil
}
