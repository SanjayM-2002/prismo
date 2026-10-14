package schema

import (
	"slices"
	"strings"
	"testing"
)

const testDatasource = `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}
`

func TestValidateValid(t *testing.T) {
	src := testDatasource + `
model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?  @default("anonymous")
  age       Int      @default(-1)
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
}

model Post {
  id    Int    @id
  title String
}
`
	s, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if err := Validate(s); err != nil {
		t.Errorf("Validate returned error: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantErrs []string
	}{
		{
			name:     "no datasource",
			src:      "model User {\n  id Int @id\n}",
			wantErrs: []string{"schema has no datasource block"},
		},
		{
			name: "datasource without provider and url",
			src:  "datasource db {\n}\nmodel User {\n  id Int @id\n}",
			wantErrs: []string{
				"line 1: datasource db has no provider",
				"line 1: datasource db has no url",
			},
		},
		{
			name:     "duplicate model",
			src:      testDatasource + "model User {\n  id Int @id\n}\nmodel User {\n  id Int @id\n}",
			wantErrs: []string{`line 8: duplicate model "User"`},
		},
		{
			name:     "model without id",
			src:      testDatasource + "model User {\n  email String\n}",
			wantErrs: []string{"line 5: model User has no @id field"},
		},
		{
			name:     "model with two ids",
			src:      testDatasource + "model User {\n  id Int @id\n  code Int @id\n}",
			wantErrs: []string{"line 5: model User has more than one @id field"},
		},
		{
			name:     "duplicate field",
			src:      testDatasource + "model User {\n  id Int @id\n  email String\n  email String\n}",
			wantErrs: []string{`line 8: duplicate field "email" in model User`},
		},
		{
			name:     "optional id",
			src:      testDatasource + "model User {\n  id Int? @id\n}",
			wantErrs: []string{"line 6: @id field id cannot be optional"},
		},
		{
			name:     "autoincrement on a non-id field",
			src:      testDatasource + "model User {\n  id Int @id\n  count Int @default(autoincrement())\n}",
			wantErrs: []string{"line 7: autoincrement() is only allowed on an Int @id field"},
		},
		{
			name:     "autoincrement on a String id",
			src:      testDatasource + "model User {\n  id String @id @default(autoincrement())\n}",
			wantErrs: []string{"line 6: autoincrement() is only allowed on an Int @id field"},
		},
		{
			name:     "now on a non-DateTime field",
			src:      testDatasource + "model User {\n  id Int @id\n  name String @default(now())\n}",
			wantErrs: []string{"line 7: now() is only allowed on a DateTime field"},
		},
		{
			name:     "unknown function",
			src:      testDatasource + "model User {\n  id Int @id @default(uuid())\n}",
			wantErrs: []string{"line 6: unknown function uuid() in @default"},
		},
		{
			name:     "bool default on Int",
			src:      testDatasource + "model User {\n  id Int @id\n  age Int @default(true)\n}",
			wantErrs: []string{"line 7: @default(true) does not match type Int"},
		},
		{
			name:     "string default on Boolean",
			src:      testDatasource + "model User {\n  id Int @id\n  active Boolean @default(\"true\")\n}",
			wantErrs: []string{`line 7: @default("true") does not match type Boolean`},
		},
		{
			name:     "number default on String",
			src:      testDatasource + "model User {\n  id Int @id\n  name String @default(1)\n}",
			wantErrs: []string{"line 7: @default(1) does not match type String"},
		},
		{
			name:     "decimal default on Int",
			src:      testDatasource + "model User {\n  id Int @id\n  age Int @default(1.5)\n}",
			wantErrs: []string{"line 7: @default(1.5) is not a valid Int"},
		},
		{
			name:     "Int default out of range",
			src:      testDatasource + "model User {\n  id Int @id\n  age Int @default(3000000000)\n}",
			wantErrs: []string{"line 7: @default(3000000000) is not a valid Int"},
		},
		{
			name: "several problems are all reported",
			src: `datasource db {
  provider = "mysql"
}
model User {
  email String
  email String @default(true)
}
model Post {
  id Int? @id
}
`,
			wantErrs: []string{
				"line 1: datasource db has no url",
				"line 4: model User has no @id field",
				`line 6: duplicate field "email" in model User`,
				"line 6: @default(true) does not match type String",
				"line 9: @id field id cannot be optional",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Parse(tt.src)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}

			err = Validate(s)
			if err == nil {
				t.Fatalf("Validate returned no error, want %q", tt.wantErrs)
			}

			// errors.Join puts each error on its own line.
			got := strings.Split(err.Error(), "\n")
			if !slices.Equal(got, tt.wantErrs) {
				t.Errorf("Validate errors:\n got: %q\nwant: %q", got, tt.wantErrs)
			}
		})
	}
}
