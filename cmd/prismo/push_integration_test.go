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

	"github.com/SanjayM-2002/prismo/dialect/mysql"
)

const pushDatasource = `datasource db {
  provider = "mysql"
  url      = env("PRISMO_PUSH_TEST_URL")
}
`

const pushUser = `
model PushUser {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
}
`

const pushPost = `
model PushPost {
  id    Int    @id
  title String
}
`

// pushTest runs db push against PRISMO_TEST_DSN. It starts from an empty
// database, because db push drops every table that isn't in the schema.
type pushTest struct {
	t    *testing.T
	db   *sql.DB
	path string
}

func newPushTest(t *testing.T) *pushTest {
	t.Helper()
	dsn := os.Getenv("PRISMO_TEST_DSN")
	if dsn == "" {
		t.Skip("PRISMO_TEST_DSN is not set")
	}
	db, err := mysql.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Cleanups run in reverse order, so the connection closes after the tables are dropped.
	t.Cleanup(func() { db.Close() })

	pt := &pushTest{t: t, db: db, path: filepath.Join(t.TempDir(), "schema.prismo")}
	pt.dropAllTables()
	t.Cleanup(pt.dropAllTables)
	t.Setenv("PRISMO_PUSH_TEST_URL", dsn)
	return pt
}

func (pt *pushTest) dropAllTables() {
	tables, err := mysql.Dialect{}.Tables(context.Background(), pt.db)
	if err != nil {
		pt.t.Fatal(err)
	}
	for _, table := range tables {
		pt.exec("DROP TABLE " + mysql.Dialect{}.QuoteIdent(table.Name))
	}
}

func (pt *pushTest) exec(query string) {
	pt.t.Helper()
	if _, err := pt.db.ExecContext(context.Background(), query); err != nil {
		pt.t.Fatal(err)
	}
}

// push writes the schema and runs db push with the given extra flags.
func (pt *pushTest) push(models string, flags ...string) (code int, stdout, stderr string) {
	pt.t.Helper()
	if err := os.WriteFile(pt.path, []byte(pushDatasource+models), 0o644); err != nil {
		pt.t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	args := append([]string{"db", "push", "--schema", pt.path}, flags...)
	code = run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func (pt *pushTest) tableNames() []string {
	pt.t.Helper()
	tables, err := mysql.Dialect{}.Tables(context.Background(), pt.db)
	if err != nil {
		pt.t.Fatal(err)
	}
	var names []string
	for _, table := range tables {
		names = append(names, table.Name)
	}
	return names
}

func wantContains(t *testing.T, label, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", label, got, want)
		}
	}
}

func TestPushTables(t *testing.T) {
	pt := newPushTest(t)

	t.Run("creates tables in an empty database", func(t *testing.T) {
		code, stdout, stderr := pt.push(pushUser)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Create table PushUser", "Applied 1 change.")
	})

	t.Run("created table has the schema's columns and unique index", func(t *testing.T) {
		tables, err := mysql.Dialect{}.Tables(context.Background(), pt.db)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 1 {
			t.Fatalf("tables = %+v, want only PushUser", tables)
		}
		var cols []string
		for _, c := range tables[0].Columns {
			cols = append(cols, c.Name)
		}
		if want := []string{"id", "email", "name", "active", "createdAt"}; !reflect.DeepEqual(cols, want) {
			t.Errorf("columns = %q, want %q", cols, want)
		}
		if !reflect.DeepEqual(tables[0].Unique, []string{"email"}) {
			t.Errorf("unique columns = %q, want [email]", tables[0].Unique)
		}
	})

	t.Run("second push finds nothing to do", func(t *testing.T) {
		code, stdout, stderr := pt.push(pushUser)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "The database is already in sync with the schema.")
	})

	t.Run("new model creates only its table", func(t *testing.T) {
		code, stdout, stderr := pt.push(pushUser + pushPost)
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Create table PushPost", "Applied 1 change.")
		if strings.Contains(stdout, "PushUser") {
			t.Errorf("stdout = %q, PushUser should not change", stdout)
		}
	})

	t.Run("removed model is not dropped without the flag", func(t *testing.T) {
		pt.exec("INSERT INTO `PushPost` (`id`, `title`) VALUES (1, 'Hello')")

		code, stdout, stderr := pt.push(pushUser)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stdout", stdout, "Drop table PushPost", "These changes can lose data:",
			"Table PushPost and all its rows will be dropped")
		wantContains(t, "stderr", stderr, "nothing was applied; run again with --accept-data-loss")

		var count int
		if err := pt.db.QueryRow("SELECT COUNT(*) FROM `PushPost`").Scan(&count); err != nil {
			t.Fatalf("PushPost should still exist: %v", err)
		}
		if count != 1 {
			t.Errorf("PushPost has %d rows, want 1", count)
		}
	})

	t.Run("removed model is dropped with the flag", func(t *testing.T) {
		code, stdout, stderr := pt.push(pushUser, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Drop table PushPost", "Applied 1 change.")
		if got := pt.tableNames(); !reflect.DeepEqual(got, []string{"PushUser"}) {
			t.Errorf("tables = %q, want [PushUser]", got)
		}
	})

	t.Run("table not in the schema is dropped", func(t *testing.T) {
		pt.exec("CREATE TABLE `PushHandMade` (`id` INT NOT NULL PRIMARY KEY)")

		code, stdout, stderr := pt.push(pushUser, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Drop table PushHandMade")
		if got := pt.tableNames(); !reflect.DeepEqual(got, []string{"PushUser"}) {
			t.Errorf("tables = %q, want [PushUser]", got)
		}
	})
}

func TestPushColumns(t *testing.T) {
	pt := newPushTest(t)
	const base = `
model PushUser {
  id    Int    @id @default(autoincrement())
  email String @unique
`
	if code, _, stderr := pt.push(base + "}"); code != 0 {
		t.Fatalf("first push failed: %s", stderr)
	}

	t.Run("empty table: required column is added without the flag", func(t *testing.T) {
		code, stdout, stderr := pt.push(base + "  code String\n}")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Add column PushUser.code", "Applied 1 change.")
	})

	t.Run("empty table: column is dropped without the flag", func(t *testing.T) {
		code, stdout, stderr := pt.push(base + "}")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Drop column PushUser.code", "Applied 1 change.")
	})

	pt.exec("INSERT INTO `PushUser` (`email`) VALUES ('a@b.com')")

	t.Run("optional column is added to a table with rows", func(t *testing.T) {
		code, stdout, stderr := pt.push(base + "  age Int?\n}")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Add column PushUser.age", "Applied 1 change.")
	})

	t.Run("required column without default needs the flag when there are rows", func(t *testing.T) {
		models := base + "  age Int?\n  nickname String\n}"
		code, stdout, _ := pt.push(models)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stdout", stdout, "Required column PushUser.nickname has no default")

		code, stdout, stderr := pt.push(models, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Add column PushUser.nickname", "Applied 1 change.")

		var nickname string
		if err := pt.db.QueryRow("SELECT `nickname` FROM `PushUser`").Scan(&nickname); err != nil {
			t.Fatal(err)
		}
		if nickname != "" {
			t.Errorf("nickname = %q, want MySQL's empty default", nickname)
		}
	})

	t.Run("column with values needs the flag to be dropped", func(t *testing.T) {
		models := base + "  nickname String\n}"
		code, stdout, _ := pt.push(models)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stdout", stdout, "Drop column PushUser.age", "Column PushUser.age and all its values will be dropped")

		code, stdout, stderr := pt.push(models, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Drop column PushUser.age", "Applied 1 change.")
	})

	t.Run("database matches the schema afterwards", func(t *testing.T) {
		code, stdout, stderr := pt.push(base + "  nickname String\n}")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "already in sync")
	})
}

func TestPushChangedColumns(t *testing.T) {
	pt := newPushTest(t)
	model := func(fields string) string {
		return "\nmodel PushUser {\n  id Int @id @default(autoincrement())\n" + fields + "}\n"
	}

	if code, _, stderr := pt.push(model("  age Int?\n  nick String?\n")); code != 0 {
		t.Fatalf("first push failed: %s", stderr)
	}
	pt.exec("INSERT INTO `PushUser` (`age`, `nick`) VALUES (NULL, '42')")

	t.Run("new default is applied without the flag", func(t *testing.T) {
		code, stdout, stderr := pt.push(model("  age Int? @default(18)\n  nick String?\n"))
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Change column PushUser.age (default none -> 18)", "Applied 1 change.")
	})

	t.Run("optional to required needs the flag and fails on NULL rows", func(t *testing.T) {
		models := model("  age Int @default(18)\n  nick String?\n")
		code, stdout, _ := pt.push(models)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stdout", stdout, "Column PushUser.age becomes required")

		// The row has age NULL, so MySQL refuses the change even with the flag.
		code, _, stderr := pt.push(models, "--accept-data-loss")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stderr", stderr, "Change column PushUser.age", "(nothing was applied)")

		pt.exec("UPDATE `PushUser` SET `age` = 30")
		code, stdout, stderr = pt.push(models, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Applied 1 change.")
	})

	t.Run("type change needs the flag and converts values", func(t *testing.T) {
		models := model("  age Int @default(18)\n  nick Int?\n")
		code, stdout, _ := pt.push(models)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stdout", stdout, "Column PushUser.nick changes type from String to Int")

		code, _, stderr := pt.push(models, "--accept-data-loss")
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		var nick int
		if err := pt.db.QueryRow("SELECT `nick` FROM `PushUser`").Scan(&nick); err != nil {
			t.Fatal(err)
		}
		if nick != 42 {
			t.Errorf("nick = %d, want 42 converted from '42'", nick)
		}
	})

	t.Run("database matches the schema afterwards", func(t *testing.T) {
		code, stdout, stderr := pt.push(model("  age Int @default(18)\n  nick Int?\n"))
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "already in sync")
	})
}

func TestPushUniqueAndPrimaryKey(t *testing.T) {
	pt := newPushTest(t)
	model := func(fields string) string {
		return "\nmodel PushUser {\n  id Int @id @default(autoincrement())\n" + fields + "}\n"
	}

	if code, _, stderr := pt.push(model("  email String\n")); code != 0 {
		t.Fatalf("first push failed: %s", stderr)
	}

	t.Run("unique index is added", func(t *testing.T) {
		code, stdout, stderr := pt.push(model("  email String @unique\n"))
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Add unique index on PushUser.email", "Applied 1 change.")
	})

	t.Run("unique index stops duplicates", func(t *testing.T) {
		pt.exec("INSERT INTO `PushUser` (`email`) VALUES ('a@b.com')")
		if _, err := pt.db.Exec("INSERT INTO `PushUser` (`email`) VALUES ('a@b.com')"); err == nil {
			t.Errorf("duplicate email was inserted, want the unique index to stop it")
		}
	})

	t.Run("unique index is dropped", func(t *testing.T) {
		code, stdout, stderr := pt.push(model("  email String\n"))
		if code != 0 {
			t.Fatalf("exit code = %d, stderr: %s", code, stderr)
		}
		wantContains(t, "stdout", stdout, "Drop unique index on PushUser.email", "Applied 1 change.")
		pt.exec("INSERT INTO `PushUser` (`email`) VALUES ('a@b.com')")
	})

	t.Run("adding a unique index fails on duplicate values", func(t *testing.T) {
		code, _, stderr := pt.push(model("  email String @unique\n"))
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stderr", stderr, "Add unique index on PushUser.email", "Duplicate entry", "(nothing was applied)")
	})

	t.Run("changing the primary key is refused", func(t *testing.T) {
		code, _, stderr := pt.push("\nmodel PushUser {\n  id Int\n  email String @id\n}\n", "--accept-data-loss")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		wantContains(t, "stderr", stderr, `changing the primary key from "id" to "email" is not supported yet`)
	})
}
