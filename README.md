# prismo

A Prisma-inspired ORM for Go, written in Go.

You describe your models in a schema file. prismo keeps your MySQL database in
sync with it and generates a type-safe Go client for querying it.

## Schema

```prisma
datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
}
```

Types: `Int`, `String`, `Boolean`, `DateTime`, optional fields with `?`.
Attributes: `@id`, `@unique`, `@default(...)` with literals, `autoincrement()` and `now()`.

## Commands

```sh
go install github.com/SanjayM-2002/prismo/cmd/prismo@latest

prismo db push      # make the database match the schema
prismo generate     # write the Go client to ./db
```

`db push` creates, changes and drops tables, columns and unique indexes. Changes
that would lose data stop with a list of what would be lost; run again with
`--accept-data-loss` to apply them anyway.

## Client

```go
client, err := db.Open(ctx) // reads DATABASE_URL
if err != nil {
	log.Fatal(err)
}
defer client.Close()

u, err := client.User.Create(ctx, db.UserCreate{Email: "a@b.com", Name: db.Ptr("Sanjay")})
u, err = client.User.FindByEmail(ctx, "a@b.com") // db.ErrNotFound if missing
active, err := client.User.FindMany(ctx, db.UserWhere{Active: db.Ptr(true)})
n, err := client.User.Count(ctx, db.UserWhere{})

u, err = client.User.UpdateByID(ctx, u.ID, db.UserUpdate{Name: db.Ptr("Sam")})
u, err = client.User.DeleteByID(ctx, u.ID)

err = client.Tx(ctx, func(tx *db.Client) error {
	_, err := tx.User.Create(ctx, db.UserCreate{Email: "b@b.com"})
	return err // returning an error rolls everything back
})
```

## Development

```sh
go test ./...
PRISMO_TEST_DSN="user:pass@tcp(localhost:3306)/prismo_test" go test -tags integration -p 1 ./...
```

Integration tests create and drop tables, so point `PRISMO_TEST_DSN` at an
empty database used only for tests.

Supports MySQL 8.0+. Work in progress.
