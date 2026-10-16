package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var mysqlDialect = Dialect{Import: "github.com/SanjayM-2002/prismo/dialect/mysql", Name: "mysql"}

func TestClientGolden(t *testing.T) {
	tests := []struct {
		golden string
		src    string
	}{
		{golden: "client.golden", src: testSchema},
		{golden: "client_url.golden", src: strings.Replace(testSchema,
			`env("DATABASE_URL")`, `"root:pass@tcp(localhost:3306)/app"`, 1)},
	}

	for _, tt := range tests {
		t.Run(tt.golden, func(t *testing.T) {
			s := parse(t, tt.src)
			got, err := Client(s, "db", mysqlDialect)
			if err != nil {
				t.Fatalf("Client returned error: %v", err)
			}
			checkGolden(t, tt.golden, got)

			models, err := Models(s, "db")
			if err != nil {
				t.Fatalf("Models returned error: %v", err)
			}
			build(t, map[string][]byte{"models.go": models, "client.go": got})
		})
	}
}

// build writes files into a temporary package inside this module and runs
// go build on it, so imports of prismo's own packages resolve.
func build(t *testing.T, files map[string][]byte) {
	t.Helper()
	dir, err := os.MkdirTemp("testdata", "build-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("go", "build", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, out)
	}
}

func TestClientNameClash(t *testing.T) {
	const datasource = `datasource db {
  provider = "mysql"
  url      = env("DATABASE_URL")
}
`
	tests := []struct {
		name    string
		models  string
		wantErr string
	}{
		{
			name:    "model named Client",
			models:  "model Client {\n  id Int @id\n}",
			wantErr: "model Client: generated name Client is reserved by the client",
		},
		{
			name:    "model name ending in Client",
			models:  "model Post {\n  id Int @id\n}\nmodel PostClient {\n  id Int @id\n}",
			wantErr: "models Post and PostClient both generate the name PostClient",
		},
		{
			name:    "model name ending in Create",
			models:  "model User {\n  id Int @id\n}\nmodel UserCreate {\n  id Int @id\n}",
			wantErr: "models User and UserCreate both generate the name UserCreate",
		},
		{
			name:    "model name ending in Where",
			models:  "model User {\n  id Int @id\n}\nmodel UserWhere {\n  id Int @id\n}",
			wantErr: "models User and UserWhere both generate the name UserWhere",
		},
		{
			name:    "model name ending in Update",
			models:  "model User {\n  id Int @id\n}\nmodel UserUpdate {\n  id Int @id\n}",
			wantErr: "models User and UserUpdate both generate the name UserUpdate",
		},
		{
			name:    "model named Ptr",
			models:  "model Ptr {\n  id Int @id\n}",
			wantErr: "model Ptr: generated name Ptr is reserved by the client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := parse(t, datasource+tt.models)
			if _, err := Client(s, "db", mysqlDialect); err == nil || err.Error() != tt.wantErr {
				t.Errorf("Client error = %v, want %q", err, tt.wantErr)
			}
			if _, err := Queries(s, "db", mysqlDialect); err == nil || err.Error() != tt.wantErr {
				t.Errorf("Queries error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
