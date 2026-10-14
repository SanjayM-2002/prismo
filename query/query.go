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

	var args []any
	for i, cond := range q.Where {
		if i == 0 {
			b.WriteString(" WHERE ")
		} else {
			b.WriteString(" AND ")
		}
		args = append(args, cond.Value)
		fmt.Fprintf(&b, "%s = %s", d.QuoteIdent(cond.Column), d.Placeholder(len(args)))
	}

	if q.Limit > 0 {
		fmt.Fprintf(&b, " LIMIT %d", q.Limit)
	}
	return b.String(), args, nil
}

func quoteAll(d Dialect, names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = d.QuoteIdent(name)
	}
	return strings.Join(quoted, ", ")
}
