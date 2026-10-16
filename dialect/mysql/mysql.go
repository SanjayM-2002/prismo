// Package mysql holds everything MySQL-specific: SQL rendering,
// introspection and opening connections.
package mysql

import (
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

// SQL returns the statement for one migration step.
func (d Dialect) SQL(step migrate.Step) (string, error) {
	switch step := step.(type) {
	case migrate.CreateTable:
		return d.CreateTable(step)
	case migrate.DropTable:
		return fmt.Sprintf("DROP TABLE %s;", d.QuoteIdent(step.Name)), nil
	case migrate.AddColumn:
		col, err := d.column(step.Column)
		if err != nil {
			return "", fmt.Errorf("table %s: %w", step.Table, err)
		}
		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", d.QuoteIdent(step.Table), col), nil
	case migrate.AlterColumn:
		col, err := d.column(step.To)
		if err != nil {
			return "", fmt.Errorf("table %s: %w", step.Table, err)
		}
		return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;", d.QuoteIdent(step.Table), col), nil
	case migrate.DropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", d.QuoteIdent(step.Table), d.QuoteIdent(step.Column)), nil
	case migrate.AddUnique:
		return fmt.Sprintf("ALTER TABLE %s ADD UNIQUE INDEX %s (%s);", d.QuoteIdent(step.Table),
			d.QuoteIdent(uniqueIndexName(step.Table, step.Column)), d.QuoteIdent(step.Column)), nil
	case migrate.DropUnique:
		return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s;", d.QuoteIdent(step.Table),
			d.QuoteIdent(uniqueIndexName(step.Table, step.Column))), nil
	default:
		return "", fmt.Errorf("unsupported migration step %T", step)
	}
}

// CreateTable returns the CREATE TABLE statement for a step.
func (d Dialect) CreateTable(step migrate.CreateTable) (string, error) {
	t := step.Table
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
		lines = append(lines, fmt.Sprintf("  UNIQUE INDEX %s (%s)", d.QuoteIdent(uniqueIndexName(t.Name, col)), d.QuoteIdent(col)))
	}

	sql := fmt.Sprintf("CREATE TABLE %s (\n%s\n) DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
		d.QuoteIdent(t.Name), strings.Join(lines, ",\n"))
	return sql, nil
}

// uniqueIndexName names the unique index on one column the way Prisma does,
// so prismo can find it again to drop it.
func uniqueIndexName(table, column string) string {
	return table + "_" + column + "_key"
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
