package mysql

import (
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
)

func TestConnectionDSN(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "no options", url: "root:pass@tcp(localhost:3306)/app"},
		{name: "keeps other options", url: "root:pass@tcp(localhost:3306)/app?timeout=5s"},
		{name: "already set", url: "root:pass@tcp(localhost:3306)/app?parseTime=true"},
		{name: "explicitly off", url: "root:pass@tcp(localhost:3306)/app?parseTime=false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, err := connectionDSN(tt.url)
			if err != nil {
				t.Fatalf("connectionDSN returned error: %v", err)
			}

			cfg, err := driver.ParseDSN(dsn)
			if err != nil {
				t.Fatalf("result %q does not parse: %v", dsn, err)
			}
			if !cfg.ParseTime {
				t.Errorf("parseTime is off in %q", dsn)
			}
			if !cfg.ClientFoundRows {
				t.Errorf("clientFoundRows is off in %q", dsn)
			}
			if cfg.DBName != "app" || cfg.User != "root" || cfg.Addr != "localhost:3306" {
				t.Errorf("connection details changed: %q", dsn)
			}
		})
	}

	t.Run("timeout survives", func(t *testing.T) {
		dsn, err := connectionDSN("root:pass@tcp(localhost:3306)/app?timeout=5s")
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := driver.ParseDSN(dsn)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Timeout != 5*time.Second {
			t.Errorf("timeout = %v, want 5s", cfg.Timeout)
		}
	})
}

func TestConnectionDSNInvalidURL(t *testing.T) {
	_, err := connectionDSN("not a url")
	if err == nil || !strings.Contains(err.Error(), "invalid database url") {
		t.Errorf("error = %v, want an invalid database url error", err)
	}
}
