// Command prismo manages a prismo schema: it makes the database match
// schema.prismo and generates a Go client from it.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
)

const usage = `prismo is a schema-first ORM for Go.

Usage:
  prismo <command> [flags]

Commands:
  db push   Make the database match the schema
  generate  Generate the Go client from the schema
  help      Show this help

Run "prismo <command> -h" for a command's flags.
`

const dbUsage = `Usage:
  prismo db <command> [flags]

Commands:
  push  Make the database match the schema
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	case "db":
		return runDB(ctx, args[1:], stdout, stderr)
	case "generate":
		if err := generateClient(args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runDB(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, dbUsage)
		return 2
	}

	switch args[0] {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, dbUsage)
		return 0
	case "push":
		if err := push(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown db command %q\n\n%s", args[0], dbUsage)
		return 2
	}
}
