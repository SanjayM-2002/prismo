package migrate

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/SanjayM-2002/prismo/schema"
)

// Step is one change to the database.
type Step interface {
	// Describe says what the step does, e.g. "Create table Post".
	Describe() string
	// Warning says what data the step can lose, or "" if it can't lose any.
	Warning() string
}

// DropTable is a step that drops a table and all its rows.
type DropTable struct {
	Name    string
	HasRows bool
}

// AddColumn is a step that adds a column to an existing table.
type AddColumn struct {
	Table   string
	Column  Column
	HasRows bool
}

// DropColumn is a step that drops a column and all its values.
type DropColumn struct {
	Table   string
	Column  string
	HasRows bool
}

// AlterColumn is a step that changes a column's type, nullability, default
// or auto-increment.
type AlterColumn struct {
	Table   string
	From    Column
	To      Column
	HasRows bool
}

// AddUnique is a step that adds a unique index on one column.
type AddUnique struct {
	Table  string
	Column string
}

// DropUnique is a step that drops the unique index on one column.
type DropUnique struct {
	Table  string
	Column string
}

// Describe implements Step.
func (s CreateTable) Describe() string { return "Create table " + s.Table.Name }

// Warning implements Step.
func (s CreateTable) Warning() string { return "" }

// Describe implements Step.
func (s DropTable) Describe() string { return "Drop table " + s.Name }

// Warning implements Step.
func (s DropTable) Warning() string {
	if !s.HasRows {
		return ""
	}
	return fmt.Sprintf("Table %s and all its rows will be dropped", s.Name)
}

// Describe implements Step.
func (s AddColumn) Describe() string { return fmt.Sprintf("Add column %s.%s", s.Table, s.Column.Name) }

// Warning implements Step.
func (s AddColumn) Warning() string {
	c := s.Column
	if !s.HasRows || c.Nullable || c.Default != nil || c.AutoIncrement {
		return ""
	}
	return fmt.Sprintf("Required column %s.%s has no default, so existing rows get an empty value (0, '' or a zero date)", s.Table, c.Name)
}

// Describe implements Step.
func (s DropColumn) Describe() string { return fmt.Sprintf("Drop column %s.%s", s.Table, s.Column) }

// Warning implements Step.
func (s DropColumn) Warning() string {
	if !s.HasRows {
		return ""
	}
	return fmt.Sprintf("Column %s.%s and all its values will be dropped", s.Table, s.Column)
}

// Describe implements Step.
func (s AlterColumn) Describe() string {
	from, to := s.From, s.To
	var changes []string
	if from.Type != to.Type {
		changes = append(changes, fmt.Sprintf("type %s -> %s", from.Type, to.Type))
	}
	if from.Nullable != to.Nullable {
		changes = append(changes, nullability(from)+" -> "+nullability(to))
	}
	if !sameDefault(from.Default, to.Default) {
		changes = append(changes, fmt.Sprintf("default %s -> %s", defaultText(from.Default), defaultText(to.Default)))
	}
	if from.AutoIncrement != to.AutoIncrement {
		if to.AutoIncrement {
			changes = append(changes, "add autoincrement()")
		} else {
			changes = append(changes, "remove autoincrement()")
		}
	}
	return fmt.Sprintf("Change column %s.%s (%s)", s.Table, to.Name, strings.Join(changes, ", "))
}

// Warning implements Step.
func (s AlterColumn) Warning() string {
	if !s.HasRows {
		return ""
	}
	var warnings []string
	if s.From.Type != s.To.Type {
		warnings = append(warnings, fmt.Sprintf("Column %s.%s changes type from %s to %s; values that don't convert are lost or make the change fail",
			s.Table, s.To.Name, s.From.Type, s.To.Type))
	}
	if s.From.Nullable && !s.To.Nullable {
		warnings = append(warnings, fmt.Sprintf("Column %s.%s becomes required; the change fails if any row has NULL in it",
			s.Table, s.To.Name))
	}
	return strings.Join(warnings, "; ")
}

// Describe implements Step.
func (s AddUnique) Describe() string {
	return fmt.Sprintf("Add unique index on %s.%s", s.Table, s.Column)
}

// Warning implements Step. Adding the index fails if values repeat, but it
// never loses data.
func (s AddUnique) Warning() string { return "" }

// Describe implements Step.
func (s DropUnique) Describe() string {
	return fmt.Sprintf("Drop unique index on %s.%s", s.Table, s.Column)
}

// Warning implements Step.
func (s DropUnique) Warning() string { return "" }

func nullability(c Column) string {
	if c.Nullable {
		return "optional"
	}
	return "required"
}

func defaultText(d *schema.Default) string {
	switch {
	case d == nil:
		return "none"
	case d.Kind == schema.DefaultFunc:
		return d.Value + "()"
	case d.Kind == schema.DefaultString:
		return strconv.Quote(d.Value)
	default:
		return d.Value
	}
}

func sameColumn(a, b Column) bool {
	return a.Type == b.Type && a.Nullable == b.Nullable && a.AutoIncrement == b.AutoIncrement &&
		sameDefault(a.Default, b.Default)
}

func sameDefault(a, b *schema.Default) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	// The database may write a number differently than the schema: 007 vs 7.
	if a.Kind == schema.DefaultNumber {
		x, errX := strconv.ParseInt(a.Value, 10, 64)
		y, errY := strconv.ParseInt(b.Value, 10, 64)
		if errX == nil && errY == nil {
			return x == y
		}
	}
	return a.Value == b.Value
}

// Diff returns the steps that turn the tables in have into the tables in
// want. Tables, columns and unique indexes in have that aren't in want are
// dropped. Changing a table's primary key is not supported yet.
func Diff(want, have []Table) ([]Step, error) {
	wanted := map[string]bool{}
	for _, t := range want {
		wanted[t.Name] = true
	}
	existing := map[string]Table{}
	for _, t := range have {
		existing[t.Name] = t
	}

	// Steps run in this order: drops first, so a later step never trips
	// over something that is about to go away.
	var dropUniques, dropTables, dropColumns, createTables, addColumns, alterColumns, addUniques []Step

	for _, t := range have {
		if !wanted[t.Name] {
			dropTables = append(dropTables, DropTable{Name: t.Name, HasRows: t.HasRows})
		}
	}

	for _, w := range want {
		h, ok := existing[w.Name]
		if !ok {
			createTables = append(createTables, CreateTable{Table: w})
			continue
		}
		if w.PrimaryKey != h.PrimaryKey {
			return nil, fmt.Errorf("table %s: changing the primary key from %q to %q is not supported yet",
				w.Name, h.PrimaryKey, w.PrimaryKey)
		}
		for _, c := range h.Unique {
			if !slices.Contains(w.Unique, c) {
				dropUniques = append(dropUniques, DropUnique{Table: w.Name, Column: c})
			}
		}
		for _, c := range w.Unique {
			if !slices.Contains(h.Unique, c) {
				addUniques = append(addUniques, AddUnique{Table: w.Name, Column: c})
			}
		}

		for _, hc := range h.Columns {
			if _, ok := column(w, hc.Name); !ok {
				dropColumns = append(dropColumns, DropColumn{Table: w.Name, Column: hc.Name, HasRows: h.HasRows})
			}
		}
		for _, wc := range w.Columns {
			hc, ok := column(h, wc.Name)
			switch {
			case !ok:
				addColumns = append(addColumns, AddColumn{Table: w.Name, Column: wc, HasRows: h.HasRows})
			case !sameColumn(wc, hc):
				alterColumns = append(alterColumns, AlterColumn{Table: w.Name, From: hc, To: wc, HasRows: h.HasRows})
			}
		}
	}

	return slices.Concat(dropUniques, dropTables, dropColumns, createTables, addColumns, alterColumns, addUniques), nil
}

func column(t Table, name string) (Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}
