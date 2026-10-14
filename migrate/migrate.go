// Package migrate turns a schema into database-neutral migration steps.
// A dialect renders the steps as SQL.
package migrate

import "github.com/SanjayM-2002/prismo/schema"

// CreateTable is a step that creates one table.
type CreateTable struct {
	Name    string
	Columns []Column
	// PrimaryKey is the name of the primary key column.
	PrimaryKey string
	// Unique lists the columns that get a unique index.
	Unique []string
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

// CreateTables returns a CreateTable step for every model, in schema order.
// The schema must already be validated.
func CreateTables(s *schema.Schema) []CreateTable {
	var tables []CreateTable
	for _, m := range s.Models {
		tables = append(tables, createTable(m))
	}
	return tables
}

func createTable(m schema.Model) CreateTable {
	t := CreateTable{Name: m.Name}
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
