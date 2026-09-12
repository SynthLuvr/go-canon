package testrunner

import (
	"errors"
	"strings"
	"testing"
)

func TestFakeDefaults(t *testing.T) {
	fake := &Fake{}
	code, err := fake.Run("go", "build", "./...")
	if err != nil || code != 0 {
		t.Fatalf("Run defaults = (%d, %v), want (0, nil)", code, err)
	}
	out, err := fake.Output("pandoc", "--version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "pandoc --version") {
		t.Errorf("Output default = %q, want the joined command", out)
	}
	path, err := fake.Resolve("go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, "/fake/bin/") {
		t.Errorf("Resolve default = %q, want a fake absolute path", path)
	}
	if len(fake.Commands) != 2 { // Run + Output record; Resolve does not
		t.Errorf("Commands = %v, want exactly the Run/Output calls", fake.Commands)
	}
	if fake.Commands[0] != "go build ./..." || fake.Commands[1] != "pandoc --version" {
		t.Errorf("Commands = %v, want the recorded Run/Output commands", fake.Commands)
	}
}

func TestFakeClosures(t *testing.T) {
	fake := &Fake{
		RunFn: func(name string, args []string) (int, error) {
			return 3, nil
		},
		OutputFn: func(name string, args []string) ([]byte, error) {
			return nil, errors.New("boom")
		},
		ResolveFn: func(name string) (string, error) {
			return "", errors.New("missing")
		},
	}
	if code, _ := fake.Run("modernize"); code != 3 {
		t.Errorf("Run = %d, want 3", code)
	}
	if _, err := fake.Output("pandoc"); err == nil {
		t.Error("Output succeeded, want error")
	}
	if _, err := fake.Resolve("task"); err == nil {
		t.Error("Resolve succeeded, want error")
	}
}

func TestHas(t *testing.T) {
	fake := &Fake{}
	fake.Commands = []string{"go build ./...", "golangci-lint run --config x"}
	if !fake.Has("golangci-lint run") {
		t.Error("Has(golangci-lint run) = false, want true")
	}
	if fake.Has("govulncheck") {
		t.Error("Has(govulncheck) = true, want false")
	}
	if !fake.Has("go") {
		t.Error("Has(go) = false, want prefix match")
	}
}
