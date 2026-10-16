package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/SanjayM-2002/prismo/migrate"
	"github.com/SanjayM-2002/prismo/schema"
)

// Tables reads every table in the connected database, in name order.
//
// Non-unique and multi-column unique indexes are left out because the schema
// can't express them yet. A multi-column primary key is reported as "a, b".
func (Dialect) Tables(ctx context.Context, db *sql.DB) ([]migrate.Table, error) {
	tables, err := readColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := readIndexes(ctx, db, tables); err != nil {
		return nil, err
	}
	for i := range tables {
		if tables[i].HasRows, err = hasRows(ctx, db, tables[i].Name); err != nil {
			return nil, err
		}
	}
	return tables, nil
}

func hasRows(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1 FROM "+Dialect{}.QuoteIdent(table)+" LIMIT 1").Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check rows in %s: %w", table, err)
	}
	return true, nil
}

func readColumns(ctx context.Context, db *sql.DB) ([]migrate.Table, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT c.TABLE_NAME, c.COLUMN_NAME, c.COLUMN_TYPE, c.IS_NULLABLE, c.COLUMN_DEFAULT, c.EXTRA
		FROM information_schema.COLUMNS c
		JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = c.TABLE_SCHEMA AND t.TABLE_NAME = c.TABLE_NAME
		WHERE c.TABLE_SCHEMA = DATABASE() AND t.TABLE_TYPE = 'BASE TABLE'
		ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION`)
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	defer rows.Close()

	var tables []migrate.Table
	for rows.Next() {
		var table, name, columnType, nullable, extra string
		var def sql.NullString
		if err := rows.Scan(&table, &name, &columnType, &nullable, &def, &extra); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}

		if len(tables) == 0 || tables[len(tables)-1].Name != table {
			tables = append(tables, migrate.Table{Name: table})
		}
		var defPtr *string
		if def.Valid {
			defPtr = &def.String
		}
		t := &tables[len(tables)-1]
		t.Columns = append(t.Columns, columnFromMySQL(name, columnType, nullable, defPtr, extra))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read columns: %w", err)
	}
	return tables, nil
}

func readIndexes(ctx context.Context, db *sql.DB, tables []migrate.Table) error {
	rows, err := db.QueryContext(ctx, `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`)
	if err != nil {
		return fmt.Errorf("query indexes: %w", err)
	}
	defer rows.Close()

	type index struct {
		table   string
		name    string
		unique  bool
		columns []string
	}
	var indexes []*index
	for rows.Next() {
		var table, name, column string
		var nonUnique int
		if err := rows.Scan(&table, &name, &nonUnique, &column); err != nil {
			return fmt.Errorf("scan index: %w", err)
		}
		if n := len(indexes); n == 0 || indexes[n-1].table != table || indexes[n-1].name != name {
			indexes = append(indexes, &index{table: table, name: name, unique: nonUnique == 0})
		}
		last := indexes[len(indexes)-1]
		last.columns = append(last.columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read indexes: %w", err)
	}

	byName := map[string]*migrate.Table{}
	for i := range tables {
		byName[tables[i].Name] = &tables[i]
	}
	unique := map[string]map[string]bool{}
	for _, idx := range indexes {
		t, ok := byName[idx.table]
		if !ok {
			continue
		}
		switch {
		case idx.name == "PRIMARY":
			t.PrimaryKey = strings.Join(idx.columns, ", ")
		case idx.unique && len(idx.columns) == 1:
			if unique[t.Name] == nil {
				unique[t.Name] = map[string]bool{}
			}
			unique[t.Name][idx.columns[0]] = true
		}
	}

	// List unique columns in column order, the same order the schema gives them.
	for i := range tables {
		t := &tables[i]
		for _, c := range t.Columns {
			if unique[t.Name][c.Name] {
				t.Unique = append(t.Unique, c.Name)
			}
		}
	}
	return nil
}

// columnFromMySQL turns one row of information_schema.COLUMNS into a Column.
// Types prismo doesn't create are kept as MySQL's own text, so they never
// match a schema type.
func columnFromMySQL(name, columnType, nullable string, def *string, extra string) migrate.Column {
	col := migrate.Column{
		Name:          name,
		Type:          fieldType(columnType),
		Nullable:      nullable == "YES",
		AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
	}
	if def != nil {
		col.Default = defaultFromMySQL(col.Type, *def, extra)
	}
	return col
}

func fieldType(columnType string) schema.FieldType {
	switch strings.ToLower(columnType) {
	// MySQL before 8.0.19 reports a display width: int(11).
	case "int", "int(11)":
		return schema.Int
	case "varchar(191)":
		return schema.String
	case "tinyint(1)":
		return schema.Boolean
	case "datetime(3)":
		return schema.DateTime
	default:
		return schema.FieldType(columnType)
	}
}

func defaultFromMySQL(typ schema.FieldType, value, extra string) *schema.Default {
	if strings.Contains(strings.ToUpper(extra), "DEFAULT_GENERATED") {
		switch strings.ToUpper(value) {
		case "CURRENT_TIMESTAMP(3)", "CURRENT_TIMESTAMP":
			return &schema.Default{Kind: schema.DefaultFunc, Value: "now"}
		default:
			return &schema.Default{Kind: schema.DefaultFunc, Value: value}
		}
	}

	switch typ {
	case schema.Boolean:
		switch value {
		case "1":
			return &schema.Default{Kind: schema.DefaultBool, Value: "true"}
		case "0":
			return &schema.Default{Kind: schema.DefaultBool, Value: "false"}
		}
		return &schema.Default{Kind: schema.DefaultNumber, Value: value}
	case schema.Int:
		return &schema.Default{Kind: schema.DefaultNumber, Value: value}
	default:
		return &schema.Default{Kind: schema.DefaultString, Value: value}
	}
}
