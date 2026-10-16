// Package migrate describes tables in a database-neutral way and the steps
// that change them. A dialect renders the steps as SQL.
package migrate

import "github.com/SanjayM-2002/prismo/schema"

// Table describes a table: what the schema wants, or what the database has.
type Table struct {
	Name    string
	Columns []Column
	// PrimaryKey is the name of the primary key column.
	PrimaryKey string
	// Unique lists the columns that get a unique index.
	Unique []string
	// HasRows is true when the table in the database holds at least one row.
	// It is always false for tables built from the schema.
	HasRows bool
}

// Column is a table column.
type Column struct {
	Name          string
	Type          schema.FieldType
	Nullable      bool
	AutoIncrement bool
	// Default is nil if the column has no default. It is never
	// autoincrement(); that is AutoIncrement instead.
	Default *schema.Default
}

// CreateTable is a step that creates one table.
type CreateTable struct {
	Table Table
}

// Tables returns the table every model needs, in schema order. The schema
// must already be validated.
func Tables(s *schema.Schema) []Table {
	var tables []Table
	for _, m := range s.Models {
		tables = append(tables, tableFor(m))
	}
	return tables
}

func tableFor(m schema.Model) Table {
	t := Table{Name: m.Name}
	for _, f := range m.Fields {
		col := Column{
			Name:     f.Name,
			Type:     f.Type,
			Nullable: f.Optional,
			Default:  f.Default,
		}
		// Databases treat auto-increment as a column property, not a default value.
		if f.Default != nil && f.Default.Kind == schema.DefaultFunc && f.Default.Value == "autoincrement" {
			col.AutoIncrement = true
			col.Default = nil
		}
		t.Columns = append(t.Columns, col)

		if f.ID {
			t.PrimaryKey = f.Name
		}
		if f.Unique {
			t.Unique = append(t.Unique, f.Name)
		}
	}
	return t
}
