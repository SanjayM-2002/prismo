// Package mysql holds everything MySQL-specific: SQL rendering,
// introspection and opening connections.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

// Dialect is the MySQL dialect.
type Dialect struct{}

// QuoteIdent quotes a table or column name, e.g. User -> `User`.
func (Dialect) QuoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// Placeholder returns the placeholder for a query value. MySQL uses ? for
// every position.
func (Dialect) Placeholder(n int) string {
	return "?"
}

// TableNames returns the names of the tables in the connected database.
func (Dialect) TableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'")
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tables: %w", err)
	}
	return names, nil
}

// CreateTable returns the CREATE TABLE statement for t.
func (d Dialect) CreateTable(t migrate.CreateTable) (string, error) {
	var lines []string

	for _, c := range t.Columns {
		col, err := d.column(c)
		if err != nil {
			return "", fmt.Errorf("table %s: %w", t.Name, err)
		}
		lines = append(lines, "  "+col)
	}

	if t.PrimaryKey != "" {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", d.QuoteIdent(t.PrimaryKey)))
	}

	for _, col := range t.Unique {
		index := t.Name + "_" + col + "_key"
		lines = append(lines, fmt.Sprintf("  UNIQUE INDEX %s (%s)", d.QuoteIdent(index), d.QuoteIdent(col)))
	}

	sql := fmt.Sprintf("CREATE TABLE %s (\n%s\n) DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		d.QuoteIdent(t.Name), strings.Join(lines, ",\n"))
	return sql, nil
}

func (d Dialect) column(c migrate.Column) (string, error) {
	typ, err := columnType(c.Type)
	if err != nil {
		return "", fmt.Errorf("column %s: %w", c.Name, err)
	}

	sql := d.QuoteIdent(c.Name) + " " + typ
	if c.Nullable {
		sql += " NULL"
	} else {
		sql += " NOT NULL"
	}
	if c.AutoIncrement {
		sql += " AUTO_INCREMENT"
	}
	if c.Default != nil {
		value, err := defaultValue(c.Default)
		if err != nil {
			return "", fmt.Errorf("column %s: %w", c.Name, err)
		}
		sql += " DEFAULT " + value
	}
	return sql, nil
}

func columnType(t schema.FieldType) (string, error) {
	switch t {
	case schema.Int:
		return "INT", nil
	case schema.String:
		// 191 characters keeps a utf8mb4 index under MySQL's old 767-byte key limit.
		return "VARCHAR(191)", nil
	case schema.Boolean:
		return "BOOLEAN", nil
	case schema.DateTime:
		// Millisecond precision, so values round-trip with Go's time.Time.
		return "DATETIME(3)", nil
	default:
		return "", fmt.Errorf("unsupported type %s", t)
	}
}

func defaultValue(d *schema.Default) (string, error) {
	switch d.Kind {
	case schema.DefaultString:
		return quoteString(d.Value), nil
	case schema.DefaultNumber, schema.DefaultBool:
		return d.Value, nil
	case schema.DefaultFunc:
		if d.Value == "now" {
			return "CURRENT_TIMESTAMP(3)", nil
		}
		return "", fmt.Errorf("unsupported default function %s()", d.Value)
	default:
		return "", fmt.Errorf("unsupported default kind %d", d.Kind)
	}
}

var stringEscaper = strings.NewReplacer(`\`, `\\`, `'`, `''`)

// quoteString writes s as a MySQL string literal. DDL can't use placeholders,
// so schema defaults are the one place values are written into SQL.
func quoteString(s string) string {
	return "'" + stringEscaper.Replace(s) + "'"
}
