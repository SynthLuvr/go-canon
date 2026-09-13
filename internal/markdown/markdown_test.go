package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SynthLuvlr/gocanon/internal/testrunner"
)

// writeTree creates files under root from a name → content map.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFilesWalksAndSkips(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"README.md":           "# hi\n",
		"docs/guide.md":       "# guide\n",
		"vendor/lib.md":       "# vendor\n",
		"testdata/fixture.md": "# fixture\n",
		".go-canon/gen.md":    "# gen\n",
		"src/code.go":         "// code\n",
		"notes.txt":           "not markdown\n",
	})
	got, err := Files(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "guide.md"),
	}
	if len(got) != len(want) {
		t.Fatalf("Files() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Files()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFilesExcludeGlobs(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"README.md":    "# hi\n",
		"docs/old.md":  "# old\n",
		"docs/kept.md": "# kept\n",
	})
	got, err := Files(root, []string{"docs/old.md", "docs/*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != filepath.Join(root, "README.md") {
		t.Errorf("Files(exclude docs) = %v, want only README.md", got)
	}
}

func TestFilesMissingRoot(t *testing.T) {
	if _, err := Files(filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Error("Files(missing root) succeeded, want error")
	}
}

// pandocEcho fakes pandoc: renders the named file verbatim.
func pandocEcho(name string, args []string) ([]byte, error) {
	return os.ReadFile(args[len(args)-1]) // #nosec G304 -- test-provided fixture path
}

func TestCheckClean(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"README.md": "# hi\n"})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	drifted, err := Check(fake, []string{filepath.Join(root, "README.md")})
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 0 {
		t.Errorf("Check() = %v, want no drift", drifted)
	}
	if !fake.Has("pandoc --eol=lf -t gfm") {
		t.Errorf("pandoc not invoked as expected: %v", fake.Commands)
	}
}

func TestCheckDetectsDrift(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"README.md": "# hi\n"})
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			return []byte("# canonical\n"), nil
		},
	}
	drifted, err := Check(fake, []string{filepath.Join(root, "README.md")})
	if err != nil {
		t.Fatal(err)
	}
	if len(drifted) != 1 || drifted[0] != filepath.Join(root, "README.md") {
		t.Errorf("Check() = %v, want the drifted README", drifted)
	}
}

func TestFormatRewrites(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "README.md")
	writeTree(t, root, map[string]string{"README.md": "# messy\n"})
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			return []byte("# canonical\n"), nil
		},
	}
	if err := Format(fake, []string{file}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file) // #nosec G304 -- the file just written by the test
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# canonical\n" {
		t.Errorf("Format() left %q, want the canonical rendering", got)
	}
}

func TestFormatMissingFile(t *testing.T) {
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	if err := Format(fake, []string{filepath.Join(t.TempDir(), "missing.md")}); err == nil {
		t.Error("Format(missing file) succeeded, want error")
	}
}

func TestCheckPandocMissing(t *testing.T) {
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	}
	_, err := Check(fake, []string{filepath.Join(t.TempDir(), "README.md")})
	if err == nil {
		t.Fatal("Check() without pandoc succeeded, want error")
	}
	if !strings.Contains(err.Error(), "pandoc") {
		t.Errorf("error should mention pandoc, got: %v", err)
	}
}
