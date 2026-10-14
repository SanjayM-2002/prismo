package generate

import (
	"bytes"
	"flag"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SanjayM-2002/prismo/schema"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

const testSchema = `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?
  active    Boolean  @default(true)
  age       Int?
  createdAt DateTime @default(now())
  deletedAt DateTime?
  avatarURL String?
  api_key   String
}

model Post {
  id       Int    @id
  title    String
  authorId Int
}
`

func parse(t *testing.T, src string) *schema.Schema {
	t.Helper()
	s, err := schema.Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := schema.Validate(s); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	return s
}

func TestModelsGolden(t *testing.T) {
	got, err := Models(parse(t, testSchema), "db")
	if err != nil {
		t.Fatalf("Models returned error: %v", err)
	}

	checkGolden(t, "models.golden", got)
	typeCheck(t, got)
}

// checkGolden compares got with testdata/name, or rewrites the file with -update.
func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	golden := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("generated code differs from %s (run with -update if the change is intended)\n got:\n%s\nwant:\n%s", golden, got, want)
	}
}

// typeCheck fails the test if src is not valid, compilable Go.
func typeCheck(t *testing.T, src []byte) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "models.go", src, 0)
	if err != nil {
		t.Fatalf("generated code does not parse: %v", err)
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("db", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated code does not compile: %v", err)
	}
}

func TestModelsWithoutDateTime(t *testing.T) {
	src := `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model Tag {
  id   Int    @id
  name String
}
`
	got, err := Models(parse(t, src), "db")
	if err != nil {
		t.Fatalf("Models returned error: %v", err)
	}
	if strings.Contains(string(got), "import") {
		t.Errorf("generated code imports a package it doesn't need:\n%s", got)
	}
	typeCheck(t, got)
}

func TestModelsNameClash(t *testing.T) {
	src := `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}

model User {
  id      Int @id
  userId  Int
  user_id Int
}
`
	_, err := Models(parse(t, src), "db")
	want := "model User: fields userId and user_id both become Go field UserID"
	if err == nil || err.Error() != want {
		t.Errorf("Models error = %v, want %q", err, want)
	}
}

func TestGoName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "id", want: "ID"},
		{in: "x", want: "X"},
		{in: "email", want: "Email"},
		{in: "createdAt", want: "CreatedAt"},
		{in: "user_id", want: "UserID"},
		{in: "userId", want: "UserID"},
		{in: "avatarURL", want: "AvatarURL"},
		{in: "api_key", want: "APIKey"},
		{in: "User", want: "User"},
		{in: "address2", want: "Address2"},
		{in: "first__name", want: "FirstName"},
	}

	for _, tt := range tests {
		if got := goName(tt.in); got != tt.want {
			t.Errorf("goName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
