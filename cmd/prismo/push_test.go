package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These cases all fail before connecting, so they need no database.
func TestPushErrors(t *testing.T) {
	const model = `
model User {
  id Int @id
}
`
	tests := []struct {
		name    string
		schema  string
		wantErr string
	}{
		{
			name:    "parse error",
			schema:  "model User {\n  id Strin\n}",
			wantErr: `schema.prismo: line 2: unknown type "Strin"`,
		},
		{
			name:    "validation error",
			schema:  "model User {\n  id Int\n}",
			wantErr: "schema.prismo: invalid schema:\nschema has no datasource block\nline 1: model User has no @id field",
		},
		{
			name:    "unsupported provider",
			schema:  "datasource db {\n  provider = \"postgres\"\n  url = \"x\"\n}" + model,
			wantErr: `unsupported provider "postgres"`,
		},
		{
			name:    "env variable not set",
			schema:  "datasource db {\n  provider = \"mysql\"\n  url = env(\"PRISMO_TEST_UNSET_VARIABLE\")\n}" + model,
			wantErr: "environment variable PRISMO_TEST_UNSET_VARIABLE is not set",
		},
		{
			name:    "malformed url",
			schema:  "datasource db {\n  provider = \"mysql\"\n  url = \"not a dsn\"\n}" + model,
			wantErr: "invalid database url: invalid DSN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "schema.prismo")
			if err := os.WriteFile(path, []byte(tt.schema), 0o644); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"db", "push", "--schema", path}, &stdout, &stderr)

			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

func TestPushMissingSchemaFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.prismo")

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"db", "push", "--schema", path}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read schema:") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "read schema:")
	}
}

func TestPushHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"db", "push", "-h"}, &stdout, &stderr)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "-schema") {
		t.Errorf("stderr = %q, want it to list the -schema flag", stderr.String())
	}
}
