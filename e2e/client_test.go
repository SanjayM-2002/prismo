//go:build integration

package e2e

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SanjayM-2002/prismo/dialect/mysql"
	"github.com/SanjayM-2002/prismo/e2e/db"
	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

// setup creates fresh tables for schema.prismo and returns a client opened
// the way an app would open it.
func setup(t *testing.T) *db.Client {
	t.Helper()
	dsn := os.Getenv("PRISMO_TEST_DSN")
	if dsn == "" {
		t.Skip("PRISMO_TEST_DSN is not set")
	}
	ctx := context.Background()

	conn, err := mysql.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	// Cleanups run in reverse order, so the connection closes after the tables are dropped.
	t.Cleanup(func() { conn.Close() })

	dropTables := func() {
		if _, err := conn.ExecContext(ctx, "DROP TABLE IF EXISTS `User`, `Post`"); err != nil {
			t.Fatal(err)
		}
	}
	dropTables()
	t.Cleanup(dropTables)
	createTables(t, conn)

	t.Setenv("DATABASE_URL", dsn)
	client, err := db.Open(ctx)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func createTables(t *testing.T, conn *sql.DB) {
	t.Helper()
	src, err := os.ReadFile("schema.prismo")
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range migrate.Tables(s) {
		query, err := mysql.Dialect{}.CreateTable(migrate.CreateTable{Table: table})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(context.Background(), query); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCreateFillsDefaults(t *testing.T) {
	client := setup(t)
	before := time.Now().Add(-time.Minute)

	u, err := client.User.Create(context.Background(), db.UserCreate{Email: "a@b.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if u.ID <= 0 {
		t.Errorf("ID = %d, want an id assigned by the database", u.ID)
	}
	if u.Email != "a@b.com" {
		t.Errorf("Email = %q, want a@b.com", u.Email)
	}
	if u.Name != nil {
		t.Errorf("Name = %q, want nil", *u.Name)
	}
	if !u.Active {
		t.Errorf("Active = false, want the default true")
	}
	if u.CreatedAt.Before(before) {
		t.Errorf("CreatedAt = %v, want it set to about now by the database", u.CreatedAt)
	}
}

func TestCreateWithOptionalFields(t *testing.T) {
	client := setup(t)

	u, err := client.User.Create(context.Background(), db.UserCreate{
		Email:  "a@b.com",
		Name:   db.Ptr("Sanjay"),
		Active: db.Ptr(false),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if u.Name == nil || *u.Name != "Sanjay" {
		t.Errorf("Name = %v, want Sanjay", u.Name)
	}
	if u.Active {
		t.Errorf("Active = true, want false")
	}
}

func TestCreateDuplicateUniqueField(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	if _, err := client.User.Create(ctx, db.UserCreate{Email: "a@b.com"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if _, err := client.User.Create(ctx, db.UserCreate{Email: "a@b.com"}); err == nil {
		t.Errorf("second Create with the same email succeeded, want an error")
	}
}

func TestCreateWithGivenID(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	p, err := client.Post.Create(ctx, db.PostCreate{ID: 42, Title: "Hello"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID != 42 || p.Title != "Hello" {
		t.Errorf("Create returned %+v, want ID 42 and title Hello", p)
	}

	found, err := client.Post.FindByID(ctx, 42)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if *found != *p {
		t.Errorf("FindByID = %+v, want %+v", found, p)
	}
}

func TestFindUnique(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	created, err := client.User.Create(ctx, db.UserCreate{Email: "a@b.com", Name: db.Ptr("Sanjay")})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	byID, err := client.User.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	byEmail, err := client.User.FindByEmail(ctx, "a@b.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	for _, got := range []*db.User{byID, byEmail} {
		if got.ID != created.ID || got.Email != created.Email || *got.Name != *created.Name ||
			got.Active != created.Active || !got.CreatedAt.Equal(created.CreatedAt) {
			t.Errorf("found %+v, want %+v", got, created)
		}
	}

	if _, err := client.User.FindByEmail(ctx, "nobody@b.com"); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("FindByEmail of a missing email: err = %v, want ErrNotFound", err)
	}
	if _, err := client.User.FindByID(ctx, created.ID+1000); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("FindByID of a missing id: err = %v, want ErrNotFound", err)
	}
}

func TestFindMany(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	for _, in := range []db.UserCreate{
		{Email: "a@b.com"},
		{Email: "b@b.com"},
		{Email: "c@b.com", Active: db.Ptr(false)},
	} {
		if _, err := client.User.Create(ctx, in); err != nil {
			t.Fatalf("Create %s: %v", in.Email, err)
		}
	}

	tests := []struct {
		name  string
		where db.UserWhere
		want  []string
	}{
		{name: "no filter", where: db.UserWhere{}, want: []string{"a@b.com", "b@b.com", "c@b.com"}},
		{name: "one filter", where: db.UserWhere{Active: db.Ptr(false)}, want: []string{"c@b.com"}},
		{name: "two filters", where: db.UserWhere{Active: db.Ptr(true), Email: db.Ptr("b@b.com")}, want: []string{"b@b.com"}},
		{name: "no match", where: db.UserWhere{Email: db.Ptr("nobody@b.com")}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := client.User.FindMany(ctx, tt.where)
			if err != nil {
				t.Fatalf("FindMany: %v", err)
			}
			if users == nil {
				t.Fatalf("FindMany returned nil, want an empty slice")
			}

			got := []string{}
			for _, u := range users {
				got = append(got, u.Email)
			}
			// FindMany has no ordering yet, so compare sorted.
			slices.Sort(got)
			if !slices.Equal(got, tt.want) {
				t.Errorf("emails = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDateTimeRoundTrip(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	tests := []struct {
		name string
		at   time.Time
	}{
		{name: "UTC with milliseconds", at: time.Date(2024, 1, 2, 3, 4, 5, 678_000_000, time.UTC)},
		{name: "other time zone", at: time.Date(2024, 6, 1, 23, 30, 0, 0, time.FixedZone("IST", 5*60*60+30*60))},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at := tt.at
			u, err := client.User.Create(ctx, db.UserCreate{
				Email:     fmt.Sprintf("user%d@time.com", i),
				CreatedAt: &at,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if !u.CreatedAt.Equal(tt.at) {
				t.Errorf("CreatedAt = %v, want %v", u.CreatedAt, tt.at)
			}
		})
	}
}

// createUsers creates one user per email; emails starting with "off" are inactive.
func createUsers(t *testing.T, client *db.Client, emails ...string) []*db.User {
	t.Helper()
	var users []*db.User
	for _, email := range emails {
		in := db.UserCreate{Email: email}
		if strings.HasPrefix(email, "off") {
			in.Active = db.Ptr(false)
		}
		u, err := client.User.Create(context.Background(), in)
		if err != nil {
			t.Fatalf("Create %s: %v", email, err)
		}
		users = append(users, u)
	}
	return users
}

func TestCount(t *testing.T) {
	client := setup(t)
	ctx := context.Background()
	createUsers(t, client, "a@b.com", "b@b.com", "off@b.com")

	tests := []struct {
		name  string
		where db.UserWhere
		want  int64
	}{
		{name: "all", where: db.UserWhere{}, want: 3},
		{name: "filtered", where: db.UserWhere{Active: db.Ptr(true)}, want: 2},
		{name: "no match", where: db.UserWhere{Email: db.Ptr("nobody@b.com")}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.User.Count(ctx, tt.where)
			if err != nil {
				t.Fatalf("Count: %v", err)
			}
			if got != tt.want {
				t.Errorf("Count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateByUnique(t *testing.T) {
	client := setup(t)
	ctx := context.Background()
	u, err := client.User.Create(ctx, db.UserCreate{Email: "a@b.com", Name: db.Ptr("Sanjay")})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("by id", func(t *testing.T) {
		got, err := client.User.UpdateByID(ctx, u.ID, db.UserUpdate{Name: db.Ptr("Sam"), Active: db.Ptr(false)})
		if err != nil {
			t.Fatalf("UpdateByID: %v", err)
		}
		if *got.Name != "Sam" || got.Active || got.Email != "a@b.com" {
			t.Errorf("UpdateByID returned %+v, want name Sam, inactive, email unchanged", got)
		}
	})

	t.Run("by email, changing the email itself", func(t *testing.T) {
		got, err := client.User.UpdateByEmail(ctx, "a@b.com", db.UserUpdate{Email: db.Ptr("new@b.com")})
		if err != nil {
			t.Fatalf("UpdateByEmail: %v", err)
		}
		if got.ID != u.ID || got.Email != "new@b.com" {
			t.Errorf("UpdateByEmail returned %+v, want same id with the new email", got)
		}
	})

	t.Run("clear sets NULL", func(t *testing.T) {
		got, err := client.User.UpdateByID(ctx, u.ID, db.UserUpdate{ClearName: true})
		if err != nil {
			t.Fatalf("UpdateByID: %v", err)
		}
		if got.Name != nil {
			t.Errorf("Name = %q, want nil", *got.Name)
		}
	})

	t.Run("nothing to change returns the row", func(t *testing.T) {
		got, err := client.User.UpdateByID(ctx, u.ID, db.UserUpdate{})
		if err != nil {
			t.Fatalf("UpdateByID: %v", err)
		}
		if got.ID != u.ID {
			t.Errorf("got id %d, want %d", got.ID, u.ID)
		}
	})

	t.Run("missing row", func(t *testing.T) {
		_, err := client.User.UpdateByEmail(ctx, "nobody@b.com", db.UserUpdate{Name: db.Ptr("x")})
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("duplicate unique value", func(t *testing.T) {
		createUsers(t, client, "taken@b.com")
		if _, err := client.User.UpdateByID(ctx, u.ID, db.UserUpdate{Email: db.Ptr("taken@b.com")}); err == nil {
			t.Errorf("UpdateByID to a taken email succeeded, want an error")
		}
	})
}

func TestUpdateMany(t *testing.T) {
	client := setup(t)
	ctx := context.Background()
	createUsers(t, client, "a@b.com", "b@b.com", "off@b.com")

	n, err := client.User.UpdateMany(ctx, db.UserWhere{Active: db.Ptr(true)}, db.UserUpdate{Name: db.Ptr("Active")})
	if err != nil {
		t.Fatalf("UpdateMany: %v", err)
	}
	if n != 2 {
		t.Errorf("UpdateMany = %d, want 2", n)
	}

	// Rows that already hold the new value still count as matched.
	n, err = client.User.UpdateMany(ctx, db.UserWhere{Active: db.Ptr(true)}, db.UserUpdate{Name: db.Ptr("Active")})
	if err != nil {
		t.Fatalf("UpdateMany again: %v", err)
	}
	if n != 2 {
		t.Errorf("UpdateMany with unchanged values = %d, want 2", n)
	}

	named, err := client.User.Count(ctx, db.UserWhere{Name: db.Ptr("Active")})
	if err != nil {
		t.Fatal(err)
	}
	if named != 2 {
		t.Errorf("users named Active = %d, want 2", named)
	}
}

func TestDelete(t *testing.T) {
	client := setup(t)
	ctx := context.Background()
	users := createUsers(t, client, "a@b.com", "b@b.com", "off@b.com", "off2@b.com")

	t.Run("by id returns the deleted row", func(t *testing.T) {
		got, err := client.User.DeleteByID(ctx, users[0].ID)
		if err != nil {
			t.Fatalf("DeleteByID: %v", err)
		}
		if got.Email != "a@b.com" {
			t.Errorf("DeleteByID returned %+v, want a@b.com", got)
		}
		if _, err := client.User.FindByID(ctx, users[0].ID); !errors.Is(err, db.ErrNotFound) {
			t.Errorf("FindByID after delete: err = %v, want ErrNotFound", err)
		}
	})

	t.Run("by email", func(t *testing.T) {
		if _, err := client.User.DeleteByEmail(ctx, "b@b.com"); err != nil {
			t.Fatalf("DeleteByEmail: %v", err)
		}
	})

	t.Run("missing row", func(t *testing.T) {
		if _, err := client.User.DeleteByEmail(ctx, "b@b.com"); !errors.Is(err, db.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("many", func(t *testing.T) {
		n, err := client.User.DeleteMany(ctx, db.UserWhere{Active: db.Ptr(false)})
		if err != nil {
			t.Fatalf("DeleteMany: %v", err)
		}
		if n != 2 {
			t.Errorf("DeleteMany = %d, want 2", n)
		}
		left, err := client.User.Count(ctx, db.UserWhere{})
		if err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("%d users left, want 0", left)
		}
	})
}

func TestUpsert(t *testing.T) {
	client := setup(t)
	ctx := context.Background()

	created, err := client.User.UpsertByEmail(ctx, "a@b.com",
		db.UserCreate{Email: "a@b.com", Name: db.Ptr("New")},
		db.UserUpdate{Name: db.Ptr("Updated")})
	if err != nil {
		t.Fatalf("first UpsertByEmail: %v", err)
	}
	if *created.Name != "New" {
		t.Errorf("first upsert name = %q, want New (created)", *created.Name)
	}

	updated, err := client.User.UpsertByEmail(ctx, "a@b.com",
		db.UserCreate{Email: "a@b.com", Name: db.Ptr("New")},
		db.UserUpdate{Name: db.Ptr("Updated")})
	if err != nil {
		t.Fatalf("second UpsertByEmail: %v", err)
	}
	if updated.ID != created.ID || *updated.Name != "Updated" {
		t.Errorf("second upsert = %+v, want the same row with name Updated", updated)
	}

	if n, _ := client.User.Count(ctx, db.UserWhere{}); n != 1 {
		t.Errorf("%d users, want 1", n)
	}
}

func TestTx(t *testing.T) {
	client := setup(t)
	ctx := context.Background()
	count := func() int64 {
		t.Helper()
		n, err := client.User.Count(ctx, db.UserWhere{})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("commits when fn succeeds", func(t *testing.T) {
		err := client.Tx(ctx, func(tx *db.Client) error {
			if _, err := tx.User.Create(ctx, db.UserCreate{Email: "a@b.com"}); err != nil {
				return err
			}
			_, err := tx.Post.Create(ctx, db.PostCreate{ID: 1, Title: "Hello"})
			return err
		})
		if err != nil {
			t.Fatalf("Tx: %v", err)
		}
		if count() != 1 {
			t.Errorf("user was not committed")
		}
	})

	t.Run("rolls back when fn returns an error", func(t *testing.T) {
		boom := errors.New("boom")
		err := client.Tx(ctx, func(tx *db.Client) error {
			if _, err := tx.User.Create(ctx, db.UserCreate{Email: "b@b.com"}); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("Tx error = %v, want boom", err)
		}
		if count() != 1 {
			t.Errorf("user from the failed transaction was kept")
		}
	})

	t.Run("rolls back when fn panics", func(t *testing.T) {
		func() {
			defer func() { recover() }()
			client.Tx(ctx, func(tx *db.Client) error {
				if _, err := tx.User.Create(ctx, db.UserCreate{Email: "c@b.com"}); err != nil {
					return err
				}
				panic("boom")
			})
		}()
		if count() != 1 {
			t.Errorf("user from the panicking transaction was kept")
		}
	})

	t.Run("sees its own writes", func(t *testing.T) {
		err := client.Tx(ctx, func(tx *db.Client) error {
			if _, err := tx.User.Create(ctx, db.UserCreate{Email: "d@b.com"}); err != nil {
				return err
			}
			_, err := tx.User.FindByEmail(ctx, "d@b.com")
			return err
		})
		if err != nil {
			t.Errorf("Tx: %v", err)
		}
	})

	t.Run("can't be nested or closed inside", func(t *testing.T) {
		client.Tx(ctx, func(tx *db.Client) error {
			if err := tx.Tx(ctx, func(*db.Client) error { return nil }); err == nil {
				t.Errorf("nested Tx succeeded, want an error")
			}
			if err := tx.Close(); err == nil {
				t.Errorf("Close inside Tx succeeded, want an error")
			}
			return nil
		})
	})
}
