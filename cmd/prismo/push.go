package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	_ "github.com/go-sql-driver/mysql"

	"github.com/SanjayM-2002/prismo/dialect/mysql"
	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

// dialect turns migration steps into SQL for one database.
type dialect interface {
	CreateTable(t migrate.CreateTable) (string, error)
	TableNames(ctx context.Context, db *sql.DB) ([]string, error)
}

// push creates a table for every model in the schema that doesn't have one
// yet. Existing tables are left as they are; updating them comes with migrations.
func push(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("push", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("schema", "schema.prismo", "path to the schema file")
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
	d, driver, err := dialectFor(s.Datasource.Provider)
	if err != nil {
		return err
	}
	url, err := datasourceURL(s.Datasource)
	if err != nil {
		return err
	}

	db, err := sql.Open(driver, url)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// sql.Open doesn't connect, so ping to fail early on a bad URL.
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	names, err := d.TableNames(ctx, db)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	exists := map[string]bool{}
	for _, name := range names {
		exists[name] = true
	}

	for _, t := range migrate.CreateTables(s) {
		if exists[t.Name] {
			fmt.Fprintf(stdout, "Table %s already exists, skipped (changes to existing tables are not applied yet)\n", t.Name)
			continue
		}
		query, err := d.CreateTable(t)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("create table %s: %w", t.Name, err)
		}
		fmt.Fprintf(stdout, "Created table %s\n", t.Name)
	}
	return nil
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

// dialectFor returns the dialect and database/sql driver name for a provider.
func dialectFor(provider string) (dialect, string, error) {
	switch provider {
	case "mysql":
		return mysql.Dialect{}, "mysql", nil
	default:
		return nil, "", fmt.Errorf("unsupported provider %q (supported: \"mysql\")", provider)
	}
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
