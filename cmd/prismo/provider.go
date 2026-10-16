package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/SanjayM-2002/prismo/dialect/mysql"
	"github.com/SanjayM-2002/prismo/generate"
)

// provider is what the CLI needs to work with one kind of database.
type provider struct {
	dialect dialect
	open    func(ctx context.Context, url string) (*sql.DB, error)
	// client tells the generator which dialect package generated code imports.
	client generate.Dialect
}

// providerFor returns the provider for a datasource's provider name.
func providerFor(name string) (provider, error) {
	switch name {
	case "mysql":
		return provider{
			dialect: mysql.Dialect{},
			open:    mysql.Open,
			client:  generate.Dialect{Import: "github.com/SanjayM-2002/prismo/dialect/mysql", Name: "mysql"},
		}, nil
	default:
		return provider{}, fmt.Errorf("unsupported provider %q (supported: \"mysql\")", name)
	}
}
