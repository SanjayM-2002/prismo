package mysql

import (
	"context"
	"database/sql"
	"fmt"

	driver "github.com/go-sql-driver/mysql"
)

// Open connects to the MySQL database at url and checks the connection.
// It turns on parseTime so DATETIME columns scan into time.Time.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	dsn, err := withParseTime(url)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// sql.Open doesn't connect, so ping to fail early on a bad URL.
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return db, nil
}

func withParseTime(url string) (string, error) {
	cfg, err := driver.ParseDSN(url)
	if err != nil {
		return "", fmt.Errorf("invalid database url: %w", err)
	}
	cfg.ParseTime = true
	return cfg.FormatDSN(), nil
}
