//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"testing"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PRISMO_TEST_DSN")
	if dsn == "" {
		t.Skip("PRISMO_TEST_DSN is not set")
	}
	db, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// dropTables drops the tables now and again when the test ends.
func dropTables(t *testing.T, db *sql.DB, names ...string) {
	t.Helper()
	drop := func() {
		for _, name := range names {
			if _, err := db.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+Dialect{}.QuoteIdent(name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	drop()
	t.Cleanup(drop)
}

// tablesNamed returns the tables in the database with the given names, so
// tables made by other tests don't get in the way.
func tablesNamed(t *testing.T, db *sql.DB, names ...string) []migrate.Table {
	t.Helper()
	all, err := Dialect{}.Tables(context.Background(), db)
	if err != nil {
		t.Fatalf("Tables: %v", err)
	}
	want := map[string]bool{}
	for _, name := range names {
		want[name] = true
	}
	var tables []migrate.Table
	for _, table := range all {
		if want[table.Name] {
			tables = append(tables, table)
		}
	}
	return tables
}

func TestTablesRoundTrip(t *testing.T) {
	db := openTestDB(t)
	dropTables(t, db, "IntroPost", "IntroUser")

	s, err := schema.Parse(`
model IntroPost {
  id    String @id
  title String
}

model IntroUser {
  id        Int       @id @default(autoincrement())
  email     String    @unique
  phone     String?   @unique
  name      String?   @default("it's a \\ test")
  age       Int       @default(-1)
  active    Boolean   @default(true)
  admin     Boolean   @default(false)
  createdAt DateTime  @default(now())
  deletedAt DateTime?
}
`)
	if err != nil {
		t.Fatal(err)
	}
	want := migrate.Tables(s)
	for _, table := range want {
		query, err := Dialect{}.CreateTable(migrate.CreateTable{Table: table})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}

	got := tablesNamed(t, db, "IntroPost", "IntroUser")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tables after creating them from the schema\n got: %+v\nwant: %+v", got, want)
	}
}

func TestTablesNotCreatedByPrismo(t *testing.T) {
	db := openTestDB(t)
	dropTables(t, db, "IntroForeign", "IntroPair")

	for _, query := range []string{
		"CREATE TABLE `IntroForeign` (" +
			"`id` BIGINT NOT NULL, `code` VARCHAR(255) NOT NULL, `a` INT NOT NULL, `b` INT NOT NULL, " +
			"PRIMARY KEY (`id`), INDEX `IntroForeign_code_idx` (`code`), UNIQUE INDEX `IntroForeign_a_b_key` (`a`, `b`))",
		"CREATE TABLE `IntroPair` (`a` INT NOT NULL, `b` INT NOT NULL, PRIMARY KEY (`a`, `b`))",
	} {
		if _, err := db.ExecContext(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}

	want := []migrate.Table{
		{
			Name: "IntroForeign",
			Columns: []migrate.Column{
				{Name: "id", Type: schema.FieldType("bigint")},
				{Name: "code", Type: schema.FieldType("varchar(255)")},
				{Name: "a", Type: schema.Int},
				{Name: "b", Type: schema.Int},
			},
			PrimaryKey: "id",
		},
		{
			Name: "IntroPair",
			Columns: []migrate.Column{
				{Name: "a", Type: schema.Int},
				{Name: "b", Type: schema.Int},
			},
			PrimaryKey: "a, b",
		},
	}

	got := tablesNamed(t, db, "IntroForeign", "IntroPair")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tables\n got: %+v\nwant: %+v", got, want)
	}
}
