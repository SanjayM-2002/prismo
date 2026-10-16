// Package e2e runs a generated client against a real database. The client in
// db/ is generated from schema.prismo and checked in, so the tests use it the
// way an app would.
package e2e

//go:generate go run ../cmd/prismo generate --schema schema.prismo --out db
