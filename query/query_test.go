package query_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/SanjayM-2002/prismo/dialect/mysql"
	"github.com/SanjayM-2002/prismo/query"
)

// numberedDialect quotes and numbers placeholders the way Postgres does, to
// check that query has no MySQL-specific logic.
type numberedDialect struct{}

func (numberedDialect) QuoteIdent(name string) string { return `"` + name + `"` }
func (numberedDialect) Placeholder(n int) string      { return "$" + strconv.Itoa(n) }

func TestInsertSQL(t *testing.T) {
	tests := []struct {
		name     string
		dialect  query.Dialect
		insert   query.Insert
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "one column",
			dialect:  mysql.Dialect{},
			insert:   query.Insert{Table: "User", Columns: []string{"email"}, Values: []any{"a@b.com"}},
			wantSQL:  "INSERT INTO `User` (`email`) VALUES (?)",
			wantArgs: []any{"a@b.com"},
		},
		{
			name:     "several columns",
			dialect:  mysql.Dialect{},
			insert:   query.Insert{Table: "User", Columns: []string{"email", "name", "active"}, Values: []any{"a@b.com", "Sanjay", true}},
			wantSQL:  "INSERT INTO `User` (`email`, `name`, `active`) VALUES (?, ?, ?)",
			wantArgs: []any{"a@b.com", "Sanjay", true},
		},
		{
			name:     "numbered placeholders",
			dialect:  numberedDialect{},
			insert:   query.Insert{Table: "User", Columns: []string{"email", "name"}, Values: []any{"a@b.com", nil}},
			wantSQL:  `INSERT INTO "User" ("email", "name") VALUES ($1, $2)`,
			wantArgs: []any{"a@b.com", nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.insert.SQL(tt.dialect)
			if err != nil {
				t.Fatalf("SQL returned error: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestSelectSQL(t *testing.T) {
	tests := []struct {
		name     string
		dialect  query.Dialect
		sel      query.Select
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "no filter",
			dialect:  mysql.Dialect{},
			sel:      query.Select{Table: "User", Columns: []string{"id", "email"}},
			wantSQL:  "SELECT `id`, `email` FROM `User`",
			wantArgs: nil,
		},
		{
			name:     "one filter with limit",
			dialect:  mysql.Dialect{},
			sel:      query.Select{Table: "User", Columns: []string{"id", "email"}, Where: []query.Equal{{Column: "email", Value: "a@b.com"}}, Limit: 1},
			wantSQL:  "SELECT `id`, `email` FROM `User` WHERE `email` = ? LIMIT 1",
			wantArgs: []any{"a@b.com"},
		},
		{
			name:    "several filters",
			dialect: mysql.Dialect{},
			sel: query.Select{Table: "User", Columns: []string{"id"}, Where: []query.Equal{
				{Column: "active", Value: true},
				{Column: "age", Value: 30},
			}},
			wantSQL:  "SELECT `id` FROM `User` WHERE `active` = ? AND `age` = ?",
			wantArgs: []any{true, 30},
		},
		{
			name:    "numbered placeholders",
			dialect: numberedDialect{},
			sel: query.Select{Table: "User", Columns: []string{"id"}, Where: []query.Equal{
				{Column: "active", Value: true},
				{Column: "age", Value: 30},
			}, Limit: 10},
			wantSQL:  `SELECT "id" FROM "User" WHERE "active" = $1 AND "age" = $2 LIMIT 10`,
			wantArgs: []any{true, 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.sel.SQL(tt.dialect)
			if err != nil {
				t.Fatalf("SQL returned error: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestSQLErrors(t *testing.T) {
	tests := []struct {
		name    string
		build   func() (string, []any, error)
		wantErr string
	}{
		{
			name:    "insert without columns",
			build:   func() (string, []any, error) { return query.Insert{Table: "User"}.SQL(mysql.Dialect{}) },
			wantErr: "insert into User: no columns",
		},
		{
			name: "insert with mismatched values",
			build: func() (string, []any, error) {
				return query.Insert{Table: "User", Columns: []string{"a", "b"}, Values: []any{1}}.SQL(mysql.Dialect{})
			},
			wantErr: "insert into User: 2 columns but 1 values",
		},
		{
			name:    "select without columns",
			build:   func() (string, []any, error) { return query.Select{Table: "User"}.SQL(mysql.Dialect{}) },
			wantErr: "select from User: no columns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.build()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestWriteSQL(t *testing.T) {
	tests := []struct {
		name     string
		dialect  query.Dialect
		build    func(d query.Dialect) (string, []any, error)
		wantSQL  string
		wantArgs []any
	}{
		{
			name:    "count all",
			dialect: mysql.Dialect{},
			build:   query.Count{Table: "User"}.SQL,
			wantSQL: "SELECT COUNT(*) FROM `User`",
		},
		{
			name:     "count with filter",
			dialect:  mysql.Dialect{},
			build:    query.Count{Table: "User", Where: []query.Equal{{Column: "active", Value: true}}}.SQL,
			wantSQL:  "SELECT COUNT(*) FROM `User` WHERE `active` = ?",
			wantArgs: []any{true},
		},
		{
			name:    "update one row",
			dialect: mysql.Dialect{},
			build: query.Update{
				Table: "User",
				Set:   []query.Assign{{Column: "name", Value: "Sanjay"}, {Column: "age", Value: nil}},
				Where: []query.Equal{{Column: "id", Value: 7}},
			}.SQL,
			wantSQL:  "UPDATE `User` SET `name` = ?, `age` = ? WHERE `id` = ?",
			wantArgs: []any{"Sanjay", nil, 7},
		},
		{
			name:    "update numbers placeholders across SET and WHERE",
			dialect: numberedDialect{},
			build: query.Update{
				Table: "User",
				Set:   []query.Assign{{Column: "name", Value: "Sanjay"}},
				Where: []query.Equal{{Column: "id", Value: 7}, {Column: "active", Value: true}},
			}.SQL,
			wantSQL:  `UPDATE "User" SET "name" = $1 WHERE "id" = $2 AND "active" = $3`,
			wantArgs: []any{"Sanjay", 7, true},
		},
		{
			name:     "update every row",
			dialect:  mysql.Dialect{},
			build:    query.Update{Table: "User", Set: []query.Assign{{Column: "active", Value: false}}}.SQL,
			wantSQL:  "UPDATE `User` SET `active` = ?",
			wantArgs: []any{false},
		},
		{
			name:     "delete one row",
			dialect:  mysql.Dialect{},
			build:    query.Delete{Table: "User", Where: []query.Equal{{Column: "id", Value: 7}}}.SQL,
			wantSQL:  "DELETE FROM `User` WHERE `id` = ?",
			wantArgs: []any{7},
		},
		{
			name:    "delete every row",
			dialect: mysql.Dialect{},
			build:   query.Delete{Table: "User"}.SQL,
			wantSQL: "DELETE FROM `User`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args, err := tt.build(tt.dialect)
			if err != nil {
				t.Fatalf("SQL returned error: %v", err)
			}
			if sql != tt.wantSQL {
				t.Errorf("sql = %q, want %q", sql, tt.wantSQL)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}

func TestUpdateWithoutSet(t *testing.T) {
	_, _, err := query.Update{Table: "User"}.SQL(mysql.Dialect{})
	if err == nil || err.Error() != "update User: nothing to set" {
		t.Errorf("error = %v, want %q", err, "update User: nothing to set")
	}
}
