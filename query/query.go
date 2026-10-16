// Package query builds SQL for the generated client. It only builds SQL;
// running it is up to the caller.
package query

import (
	"fmt"
	"strings"
)

// Dialect is what query needs from a database.
type Dialect interface {
	QuoteIdent(name string) string
	// Placeholder returns the placeholder for the nth value, starting at 1.
	Placeholder(n int) string
}

// Insert inserts one row.
type Insert struct {
	Table   string
	Columns []string
	Values  []any
}

// Select reads rows from one table.
type Select struct {
	Table   string
	Columns []string
	// Where lists conditions that must all match.
	Where []Equal
	// Limit is the maximum number of rows; 0 means no limit.
	Limit int
}

// Equal is a column = value condition.
type Equal struct {
	Column string
	Value  any
}

// SQL returns the INSERT statement and its arguments.
func (q Insert) SQL(d Dialect) (string, []any, error) {
	if len(q.Columns) == 0 {
		return "", nil, fmt.Errorf("insert into %s: no columns", q.Table)
	}
	if len(q.Columns) != len(q.Values) {
		return "", nil, fmt.Errorf("insert into %s: %d columns but %d values", q.Table, len(q.Columns), len(q.Values))
	}

	placeholders := make([]string, len(q.Values))
	for i := range q.Values {
		placeholders[i] = d.Placeholder(i + 1)
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		d.QuoteIdent(q.Table), quoteAll(d, q.Columns), strings.Join(placeholders, ", "))
	return sql, q.Values, nil
}

// SQL returns the SELECT statement and its arguments.
func (q Select) SQL(d Dialect) (string, []any, error) {
	if len(q.Columns) == 0 {
		return "", nil, fmt.Errorf("select from %s: no columns", q.Table)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s FROM %s", quoteAll(d, q.Columns), d.QuoteIdent(q.Table))
	args := writeWhere(&b, d, q.Where, nil)
	if q.Limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", q.Limit)
	}
	return b.String(), args, nil
}

// Count counts the rows in one table.
type Count struct {
	Table string
	// Where lists conditions that must all match.
	Where []Equal
}

// SQL returns the SELECT COUNT(*) statement and its arguments.
func (q Count) SQL(d Dialect) (string, []any, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "SELECT COUNT(*) FROM %s", d.QuoteIdent(q.Table))
	args := writeWhere(&b, d, q.Where, nil)
	return b.String(), args, nil
}

// Update changes rows in one table.
type Update struct {
	Table string
	Set   []Assign
	// Where lists conditions that must all match; empty means every row.
	Where []Equal
}

// Assign is a column = value assignment. A nil Value sets the column to NULL.
type Assign struct {
	Column string
	Value  any
}

// SQL returns the UPDATE statement and its arguments.
func (q Update) SQL(d Dialect) (string, []any, error) {
	if len(q.Set) == 0 {
		return "", nil, fmt.Errorf("update %s: nothing to set", q.Table)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "UPDATE %s SET ", d.QuoteIdent(q.Table))
	var args []any
	for i, a := range q.Set {
		if i > 0 {
			b.WriteString(", ")
		}
		args = append(args, a.Value)
		fmt.Fprintf(&b, "%s = %s", d.QuoteIdent(a.Column), d.Placeholder(len(args)))
	}
	args = writeWhere(&b, d, q.Where, args)
	return b.String(), args, nil
}

// Delete removes rows from one table.
type Delete struct {
	Table string
	// Where lists conditions that must all match; empty means every row.
	Where []Equal
}

// SQL returns the DELETE statement and its arguments.
func (q Delete) SQL(d Dialect) (string, []any, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "DELETE FROM %s", d.QuoteIdent(q.Table))
	args := writeWhere(&b, d, q.Where, nil)
	return b.String(), args, nil
}

// writeWhere writes a WHERE clause for conds and returns args with the
// condition values added. Placeholders continue numbering after args.
func writeWhere(b *strings.Builder, d Dialect, conds []Equal, args []any) []any {
	for i, cond := range conds {
		if i == 0 {
			b.WriteString(" WHERE ")
		} else {
			b.WriteString(" AND ")
		}
		args = append(args, cond.Value)
		fmt.Fprintf(b, "%s = %s", d.QuoteIdent(cond.Column), d.Placeholder(len(args)))
	}
	return args
}

func quoteAll(d Dialect, names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = d.QuoteIdent(name)
	}
	return strings.Join(quoted, ", ")
}
