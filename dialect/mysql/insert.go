package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/SanjayM-2002/prismo/query"
)

// InsertID runs q and returns the auto-increment id MySQL assigned to the
// new row.
func (d Dialect) InsertID(ctx context.Context, db *sql.DB, q query.Insert) (int64, error) {
	text, args, err := q.SQL(d)
	if err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx, text, args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read insert id: %w", err)
	}
	return id, nil
}
