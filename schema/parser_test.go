package schema

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want *Schema
	}{
		{
			name: "empty file",
			src:  "",
			want: &Schema{},
		},
		{
			name: "one model",
			src: `model User {
  id    Int
  email String
}`,
			want: &Schema{Models: []Model{
				{Name: "User", Line: 1, Fields: []Field{
					{Name: "id", Type: Int, Line: 2},
					{Name: "email", Type: String, Line: 3},
				}},
			}},
		},
		{
			name: "optional field",
			src: `model User {
  name String?
}`,
			want: &Schema{Models: []Model{
				{Name: "User", Line: 1, Fields: []Field{
					{Name: "name", Type: String, Optional: true, Line: 2},
				}},
			}},
		},
		{
			name: "all field types",
			src: `model Post {
  id        Int
  title     String
  published Boolean
  createdAt DateTime
}`,
			want: &Schema{Models: []Model{
				{Name: "Post", Line: 1, Fields: []Field{
					{Name: "id", Type: Int, Line: 2},
					{Name: "title", Type: String, Line: 3},
					{Name: "published", Type: Boolean, Line: 4},
					{Name: "createdAt", Type: DateTime, Line: 5},
				}},
			}},
		},
		{
			name: "two models with comments and blank lines",
			src: `// users of the app
model User {

  id Int // primary key

}

model Post {
  id Int
}
`,
			want: &Schema{Models: []Model{
				{Name: "User", Line: 2, Fields: []Field{
					{Name: "id", Type: Int, Line: 4},
				}},
				{Name: "Post", Line: 8, Fields: []Field{
					{Name: "id", Type: Int, Line: 9},
				}},
			}},
		},
		{
			name: "attributes",
			src: `model User {
  id        Int      @id @default(autoincrement())
  email     String   @unique
  name      String?  @default("anonymous")
  nickname  String   @default("say \"hi\"")
  age       Int      @default(-1)
  active    Boolean  @default(true)
  admin     Boolean  @default(false)
  createdAt DateTime @default(now())
  code      String   @unique @default("x")
}`,
			want: &Schema{Models: []Model{
				{Name: "User", Line: 1, Fields: []Field{
					{Name: "id", Type: Int, ID: true, Default: &Default{Kind: DefaultFunc, Value: "autoincrement"}, Line: 2},
					{Name: "email", Type: String, Unique: true, Line: 3},
					{Name: "name", Type: String, Optional: true, Default: &Default{Kind: DefaultString, Value: "anonymous"}, Line: 4},
					{Name: "nickname", Type: String, Default: &Default{Kind: DefaultString, Value: `say "hi"`}, Line: 5},
					{Name: "age", Type: Int, Default: &Default{Kind: DefaultNumber, Value: "-1"}, Line: 6},
					{Name: "active", Type: Boolean, Default: &Default{Kind: DefaultBool, Value: "true"}, Line: 7},
					{Name: "admin", Type: Boolean, Default: &Default{Kind: DefaultBool, Value: "false"}, Line: 8},
					{Name: "createdAt", Type: DateTime, Default: &Default{Kind: DefaultFunc, Value: "now"}, Line: 9},
					{Name: "code", Type: String, Unique: true, Default: &Default{Kind: DefaultString, Value: "x"}, Line: 10},
				}},
			}},
		},
		{
			name: "string default that looks like a bool",
			src:  "model User {\n  flag String @default(\"true\")\n}",
			want: &Schema{Models: []Model{
				{Name: "User", Line: 1, Fields: []Field{
					{Name: "flag", Type: String, Default: &Default{Kind: DefaultString, Value: "true"}, Line: 2},
				}},
			}},
		},
		{
			name: "datasource with env url",
			src: `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}`,
			want: &Schema{Datasource: &Datasource{
				Name: "db", Provider: "mysql", URLEnv: "DATABASE_URL", Line: 1,
			}},
		},
		{
			name: "datasource with literal url",
			src: `datasource db {
  url      = "root:pass@tcp(localhost:3306)/app"
  provider = "mysql"
}`,
			want: &Schema{Datasource: &Datasource{
				Name: "db", Provider: "mysql", URL: "root:pass@tcp(localhost:3306)/app", Line: 1,
			}},
		},
		{
			name: "datasource after models",
			src: `model User {
  id Int @id
}

datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}
`,
			want: &Schema{
				Datasource: &Datasource{Name: "db", Provider: "mysql", URLEnv: "DATABASE_URL", Line: 5},
				Models: []Model{
					{Name: "User", Line: 1, Fields: []Field{
						{Name: "id", Type: Int, ID: true, Line: 2},
					}},
				},
			},
		},
		{
			name: "empty datasource",
			src:  "datasource db {\n}",
			want: &Schema{Datasource: &Datasource{Name: "db", Line: 1}},
		},
		{
			name: "model without fields",
			src:  "model Empty {\n}",
			want: &Schema{Models: []Model{
				{Name: "Empty", Line: 1},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.src)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%q)\n got: %+v\nwant: %+v", tt.src, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name:    "lexer error is returned",
			src:     "model User # {",
			wantErr: "line 1: unexpected character '#'",
		},
		{
			name:    "unknown top-level keyword",
			src:     "table User {\n}",
			wantErr: `line 1: expected a model or datasource block, got identifier "table"`,
		},
		{
			name:    "datasource without name",
			src:     "datasource {\n}",
			wantErr: "line 1: expected identifier, got '{'",
		},
		{
			name:    "datasource without closing brace",
			src:     "datasource db {\n  provider = \"mysql\"\n",
			wantErr: "line 1: datasource db is missing a closing '}'",
		},
		{
			name:    "two datasources",
			src:     "datasource a {\n}\ndatasource b {\n}",
			wantErr: "line 3: only one datasource block is allowed",
		},
		{
			name:    "unknown datasource field",
			src:     "datasource db {\n  host = \"localhost\"\n}",
			wantErr: `line 2: unknown datasource field "host"`,
		},
		{
			name:    "duplicate datasource field",
			src:     "datasource db {\n  provider = \"mysql\"\n  provider = \"mysql\"\n}",
			wantErr: `line 3: duplicate datasource field "provider"`,
		},
		{
			name:    "datasource field without equals",
			src:     "datasource db {\n  provider \"mysql\"\n}",
			wantErr: `line 2: expected '=', got string "mysql"`,
		},
		{
			name:    "provider without quotes",
			src:     "datasource db {\n  provider = mysql\n}",
			wantErr: `line 2: expected string, got identifier "mysql"`,
		},
		{
			name:    "env without quotes",
			src:     "datasource db {\n  url = env(DATABASE_URL)\n}",
			wantErr: `line 2: expected string, got identifier "DATABASE_URL"`,
		},
		{
			name:    "url with unknown function",
			src:     "datasource db {\n  url = getenv(\"X\")\n}",
			wantErr: `line 2: expected a string or env("NAME") for url, got identifier "getenv"`,
		},
		{
			name:    "two datasource fields on one line",
			src:     "datasource db {\n  provider = \"mysql\" url = \"x\"\n}",
			wantErr: `line 2: expected newline, got identifier "url"`,
		},
		{
			name:    "model without name",
			src:     "model {\n}",
			wantErr: "line 1: expected identifier, got '{'",
		},
		{
			name:    "model without opening brace",
			src:     "model User\n  id Int\n}",
			wantErr: "line 1: expected '{', got newline",
		},
		{
			name:    "model without closing brace",
			src:     "model User {\n  id Int\n",
			wantErr: "line 1: model User is missing a closing '}'",
		},
		{
			name:    "unknown type",
			src:     "model User {\n  id Int\n  email Strin\n}",
			wantErr: `line 3: unknown type "Strin"`,
		},
		{
			name:    "field without type",
			src:     "model User {\n  id\n}",
			wantErr: "line 2: expected identifier, got newline",
		},
		{
			name:    "two fields on one line",
			src:     "model User {\n  id Int name String\n}",
			wantErr: `line 2: expected newline, got identifier "name"`,
		},
		{
			name:    "unknown attribute",
			src:     "model User {\n  id Int @uniq\n}",
			wantErr: "line 2: unknown attribute @uniq",
		},
		{
			name:    "attribute without name",
			src:     "model User {\n  id Int @\n}",
			wantErr: "line 2: expected identifier, got newline",
		},
		{
			name:    "id with arguments",
			src:     "model User {\n  id Int @id(1)\n}",
			wantErr: "line 2: @id takes no arguments",
		},
		{
			name:    "unique with arguments",
			src:     "model User {\n  id Int @unique()\n}",
			wantErr: "line 2: @unique takes no arguments",
		},
		{
			name:    "duplicate id",
			src:     "model User {\n  id Int @id @id\n}",
			wantErr: "line 2: duplicate attribute @id",
		},
		{
			name:    "duplicate default",
			src:     "model User {\n  id Int @default(1) @default(2)\n}",
			wantErr: "line 2: duplicate attribute @default",
		},
		{
			name:    "default without parentheses",
			src:     "model User {\n  id Int @default\n}",
			wantErr: "line 2: expected '(', got newline",
		},
		{
			name:    "default without value",
			src:     "model User {\n  id Int @default()\n}",
			wantErr: "line 2: expected a default value, got ')'",
		},
		{
			name:    "default with two values",
			src:     "model User {\n  id Int @default(1, 2)\n}",
			wantErr: "line 2: expected ')', got ','",
		},
		{
			name:    "default not closed",
			src:     "model User {\n  id Int @default(1\n}",
			wantErr: "line 2: expected ')', got newline",
		},
		{
			name:    "default with bare identifier",
			src:     "model User {\n  id Int @default(foo)\n}",
			wantErr: `line 2: expected a default value, got identifier "foo"`,
		},
		{
			name:    "default function with arguments",
			src:     "model User {\n  id Int @default(autoincrement(1))\n}",
			wantErr: `line 2: expected ')', got number "1"`,
		},
		{
			name:    "closing brace on field line",
			src:     "model User {\n  id Int }",
			wantErr: "line 2: expected newline, got '}'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.src)
			if err == nil {
				t.Fatalf("Parse(%q) returned no error, want %q", tt.src, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Parse(%q) error = %q, want it to contain %q", tt.src, err, tt.wantErr)
			}
		})
	}
}
