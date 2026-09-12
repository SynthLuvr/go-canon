package module

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootFromFindsNearest(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "app")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := RootFrom(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("RootFrom(%s) = %q, want %q", nested, got, root)
	}
}

func TestRootFromMissingGoMod(t *testing.T) {
	if _, err := RootFrom(t.TempDir()); err == nil {
		t.Error("RootFrom without go.mod succeeded, want error")
	}
}

func TestRootFromEmptyStart(t *testing.T) {
	if _, err := RootFrom(""); err == nil {
		t.Error("RootFrom(\"\") succeeded, want error")
	}
}
