package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGeneratedClientIsUpToDate fails when db/ no longer matches what the
// generator produces for schema.prismo.
func TestGeneratedClientIsUpToDate(t *testing.T) {
	out := filepath.Join(t.TempDir(), "db")
	cmd := exec.Command("go", "run", "../cmd/prismo", "generate", "--schema", "schema.prismo", "--out", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prismo generate failed: %v\n%s", err, output)
	}

	for _, name := range []string{"models.go", "client.go", "queries.go"} {
		want, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join("db", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("db/%s is out of date; run: go generate ./e2e", name)
		}
	}
}
