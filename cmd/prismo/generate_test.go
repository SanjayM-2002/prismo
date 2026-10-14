package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const generateSchema = `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model User {
  id    Int    @id
  email String
}
`

func writeSchema(t *testing.T, dir, src string) string {
	t.Helper()
	path := filepath.Join(dir, "schema.prismo")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()
	path := writeSchema(t, dir, generateSchema)
	out := filepath.Join(dir, "internal", "store")

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"generate", "--schema", path, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}

	files := map[string][]string{
		"models.go":  {"package store", "type User struct"},
		"client.go":  {"package store", "func Open(ctx context.Context)", "User *UserClient"},
		"queries.go": {"package store", "func (c *UserClient) FindByID(ctx context.Context, id int) (*User, error)"},
	}
	for name, wants := range files {
		file := filepath.Join(out, name)
		if !strings.Contains(stdout.String(), "Wrote "+file) {
			t.Errorf("stdout = %q, want it to mention %s", stdout.String(), file)
		}

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s does not contain %q:\n%s", name, want, src)
			}
		}
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want no warnings for an env() url", stderr.String())
	}
}

func TestGenerateWarnsAboutLiteralURL(t *testing.T) {
	dir := t.TempDir()
	src := strings.Replace(generateSchema, `env("DATABASE_URL")`, `"root:pass@tcp(localhost:3306)/app"`, 1)
	path := writeSchema(t, dir, src)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"generate", "--schema", path, "--out", filepath.Join(dir, "db")}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: the database url from the schema is written into") {
		t.Errorf("stderr = %q, want a warning about the literal url", stderr.String())
	}
}

func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		out     string
		wantErr string
	}{
		{
			name:    "invalid package name",
			schema:  generateSchema,
			out:     "my-client",
			wantErr: `output folder name "my-client" is not a valid Go package name`,
		},
		{
			name:    "invalid schema",
			schema:  "model User {\n  id Int\n}",
			out:     "db",
			wantErr: "invalid schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeSchema(t, dir, tt.schema)
			out := filepath.Join(dir, tt.out)

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"generate", "--schema", path, "--out", out}, &stdout, &stderr)

			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantErr)
			}
			if _, err := os.Stat(filepath.Join(out, "models.go")); err == nil {
				t.Errorf("models.go was written despite the error")
			}
		})
	}
}
