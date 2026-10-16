package main

import (
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"os"
	"path/filepath"

	"github.com/SanjayM-2002/prismo/generate"
)

// generateClient writes the Go client for the schema into the output folder.
// The package name is the folder name.
func generateClient(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("schema", "schema.prismo", "path to the schema file")
	out := flags.String("out", "db", "folder to write the generated code to")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	pkg := filepath.Base(filepath.Clean(*out))
	if !token.IsIdentifier(pkg) {
		return fmt.Errorf("output folder name %q is not a valid Go package name", pkg)
	}

	s, err := loadSchema(*path)
	if err != nil {
		return err
	}
	p, err := providerFor(s.Datasource.Provider)
	if err != nil {
		return err
	}

	models, err := generate.Models(s, pkg)
	if err != nil {
		return err
	}
	client, err := generate.Client(s, pkg, p.client)
	if err != nil {
		return err
	}
	queries, err := generate.Queries(s, pkg, p.client)
	if err != nil {
		return err
	}

	// Generate everything before writing, so an error leaves no half-written output.
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	for _, f := range []struct {
		name string
		src  []byte
	}{
		{name: "models.go", src: models},
		{name: "client.go", src: client},
		{name: "queries.go", src: queries},
	} {
		file := filepath.Join(*out, f.name)
		if err := os.WriteFile(file, f.src, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", file, err)
		}
		fmt.Fprintf(stdout, "Wrote %s\n", file)
	}

	if s.Datasource.URLEnv == "" {
		fmt.Fprintf(stderr, "warning: the database url from the schema is written into %s; "+
			"use url = env(\"DATABASE_URL\") to keep it out of your code\n", filepath.Join(*out, "client.go"))
	}
	return nil
}
