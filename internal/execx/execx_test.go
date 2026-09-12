package execx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newReal builds a runner capturing both streams.
func newReal() (*Real, *bytes.Buffer, *bytes.Buffer) {
	var out, errBuf bytes.Buffer
	return New(&out, &errBuf), &out, &errBuf
}

func TestRealRunStreamsAndSucceeds(t *testing.T) {
	r, out, _ := newReal()
	code, err := r.Run("go", "version")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "go version") {
		t.Errorf("stdout = %q, want it to contain the go version banner", out.String())
	}
}

func TestRealRunPropagatesExitCode(t *testing.T) {
	r, _, _ := newReal()
	code, err := r.Run("go", "tool", "definitely-not-a-go-tool")
	if err != nil {
		t.Fatalf("expected exit-code propagation, got spawn error: %v", err)
	}
	if code == 0 {
		t.Error("exit code = 0, want non-zero for a failing command")
	}
}

func TestRealOutputCapturesStdout(t *testing.T) {
	r, _, _ := newReal()
	out, err := r.Output("go", "env", "GOOS")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got == "" {
		t.Error("Output(go env GOOS) is empty, want an OS name")
	}
}

func TestRealOutputFailsOnNonZeroExit(t *testing.T) {
	r, _, _ := newReal()
	if _, err := r.Output("go", "tool", "definitely-not-a-go-tool"); err == nil {
		t.Error("Output() succeeded for a failing command, want error")
	}
}

func TestRealResolveBuiltinTool(t *testing.T) {
	r, _, _ := newReal()
	path, err := r.Resolve("cover")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Resolve(cover) = %q, want an absolute path", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("resolved cover binary missing: %v", err)
	}
}

func TestRealResolveModuleTool(t *testing.T) {
	// This repo's own go.mod pins golangci-lint as a tool directive.
	r, _, _ := newReal()
	path, err := r.Resolve("golangci-lint")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Resolve(golangci-lint) = %q, want an absolute path", path)
	}
}

func TestRealResolveUnknownGoTool(t *testing.T) {
	// A module without the tool directive must fail with guidance, never
	// silently fall back to a PATH binary.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/bare\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	r, _, _ := newReal()
	_, err := r.Resolve("golangci-lint")
	if err == nil {
		t.Fatal("Resolve(tool without directive) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "go get -tool") {
		t.Errorf("error should mention `go get -tool`, got: %v", err)
	}
}

func TestRealResolvePathFallback(t *testing.T) {
	r, _, _ := newReal()
	if _, err := r.Resolve("definitely-not-on-path-xyz"); err == nil {
		t.Error("Resolve(missing PATH binary) succeeded, want error")
	}
	path, err := r.Resolve("go")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Resolve(go) = %q, want an absolute path", path)
	}
}

func TestRealResolveCaches(t *testing.T) {
	r, _, _ := newReal()
	first, err := r.Resolve("cover")
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Resolve("cover")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("Resolve(cover) unstable: %q vs %q", first, second)
	}
}
