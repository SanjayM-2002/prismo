//go:build integration

package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPushIntegration(t *testing.T) {
	dsn := os.Getenv("PRISMO_TEST_DSN")
	if dsn == "" {
		t.Skip("PRISMO_TEST_DSN is not set")
	}

	ctx := context.Background()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dropTables := func() {
		if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS `PushTestUser`, `PushTestPost`"); err != nil {
			t.Fatal(err)
		}
	}
	dropTables()
	t.Cleanup(dropTables)

	t.Setenv("PRISMO_PUSH_TEST_URL", dsn)
	path := filepath.Join(t.TempDir(), "schema.prismo")
	src := `datasource db {
  provider = "mysql"
  url      = env("PRISMO_PUSH_TEST_URL")
}

model PushTestUser {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
}
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"push", "--schema", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("push exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created table PushTestUser") {
		t.Errorf("stdout = %q, want it to mention the created table", stdout.String())
	}

	t.Run("columns", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, `
			SELECT COLUMN_NAME, IS_NULLABLE
			FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
			ORDER BY ORDINAL_POSITION`, "PushTestUser")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()

		var got []string
		for rows.Next() {
			var name, nullable string
			if err := rows.Scan(&name, &nullable); err != nil {
				t.Fatal(err)
			}
			got = append(got, name+" "+nullable)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}

		want := []string{"id NO", "email NO", "name YES", "active NO", "createdAt NO"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("columns = %q, want %q", got, want)
		}
	})

	t.Run("unique index", func(t *testing.T) {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM information_schema.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ? AND NON_UNIQUE = 0`,
			"PushTestUser", "PushTestUser_email_key").Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("unique index PushTestUser_email_key not found")
		}
	})

	t.Run("second push skips existing tables and creates new ones", func(t *testing.T) {
		withPost := src + `
model PushTestPost {
  id    Int    @id
  title String
}
`
		if err := os.WriteFile(path, []byte(withPost), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		if code := run(ctx, []string{"push", "--schema", path}, &stdout, &stderr); code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
		}
		for _, want := range []string{"Table PushTestUser already exists, skipped", "Created table PushTestPost"} {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), want)
			}
		}
		if strings.Contains(stdout.String(), "Created table PushTestUser") {
			t.Errorf("stdout = %q, PushTestUser should not be created again", stdout.String())
		}
	})
}
