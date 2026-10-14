package generate

import "testing"

func TestQueriesGolden(t *testing.T) {
	s := parse(t, testSchema)

	got, err := Queries(s, "db", mysqlDialect)
	if err != nil {
		t.Fatalf("Queries returned error: %v", err)
	}
	checkGolden(t, "queries.golden", got)

	models, err := Models(s, "db")
	if err != nil {
		t.Fatal(err)
	}
	client, err := Client(s, "db", mysqlDialect)
	if err != nil {
		t.Fatal(err)
	}
	build(t, map[string][]byte{"models.go": models, "client.go": client, "queries.go": got})
}

func TestQueriesUnusualNames(t *testing.T) {
	src := `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model URL {
  id    Int      @id
  type  String   @unique
  query String   @unique
  c     Int      @unique
  at    DateTime @unique
}
`
	s := parse(t, src)
	models, err := Models(s, "db")
	if err != nil {
		t.Fatal(err)
	}
	client, err := Client(s, "db", mysqlDialect)
	if err != nil {
		t.Fatal(err)
	}
	queries, err := Queries(s, "db", mysqlDialect)
	if err != nil {
		t.Fatal(err)
	}
	build(t, map[string][]byte{"models.go": models, "client.go": client, "queries.go": queries})
}

func TestLowerName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "User", want: "user"},
		{in: "BlogPost", want: "blogPost"},
		{in: "ID", want: "id"},
		{in: "URL", want: "url"},
		{in: "HTTPServer", want: "httpServer"},
		{in: "APIKey", want: "apiKey"},
		{in: "X", want: "x"},
	}

	for _, tt := range tests {
		if got := lowerName(tt.in); got != tt.want {
			t.Errorf("lowerName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParamName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Email", want: "email"},
		{in: "ID", want: "id"},
		{in: "Type", want: "typeValue"},
		{in: "Range", want: "rangeValue"},
		{in: "C", want: "cValue"},
		{in: "Ctx", want: "ctxValue"},
		{in: "Query", want: "queryValue"},
	}

	for _, tt := range tests {
		if got := paramName(tt.in); got != tt.want {
			t.Errorf("paramName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
