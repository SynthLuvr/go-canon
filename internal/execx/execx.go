// Package execx runs external tools.
//
// Version-pinned Go tools (go.mod `tool` directives and builtins) are
// resolved through `go tool -n` and spawned by absolute path — never
// through a shell, a `.cmd`/`.ps1` shim, or a bare PATH binary — so
// consumer go.mod pins are always honored and managed Windows endpoints
// see only locally compiled binaries.
package execx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Runner is the process-execution seam: every external tool go-canon
// drives goes through it, so tests can substitute canned behavior.
type Runner interface {
	// Run executes name with args, streaming stdout and stderr to the
	// runner's writers, and returns the process exit code.
	Run(name string, args ...string) (int, error)
	// Output executes name with args and returns its standard output.
	// A non-zero exit is reported as an error.
	Output(name string, args ...string) ([]byte, error)
	// Resolve returns the absolute path of the executable behind name.
	// Go tools resolve through `go tool -n`; anything else through
	// PATH lookup.
	Resolve(name string) (string, error)
}

// goTools must come from `go tool` so the consumer's tool directives pin
// their versions; a same-named binary on PATH is never used for them.
var goTools = map[string]bool{
	"cover":         true,
	"golangci-lint": true,
	"govulncheck":   true,
	"modernize":     true,
	"task":          true,
}

// Real runs commands as direct child processes of this binary.
type Real struct {
	stdout, stderr io.Writer

	mu       sync.Mutex
	resolved map[string]string
}

// New returns a Real runner streaming child output to the given writers.
func New(stdout, stderr io.Writer) *Real {
	return &Real{stdout: stdout, stderr: stderr, resolved: map[string]string{}}
}

// Resolve implements Runner.
func (r *Real) Resolve(name string) (string, error) {
	if !goTools[name] {
		path, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("resolve %s from PATH: %w", name, err)
		}
		return path, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if path, ok := r.resolved[name]; ok {
		return path, nil
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("resolve go from PATH: %w", err)
	}
	out, err := capture(goPath, "go tool -n "+name, "tool", "-n", name)
	if err != nil {
		return "", fmt.Errorf("resolve go tool %s — is it in your go.mod tool block? (add it with `go get -tool <pkg>`): %w", name, err)
	}
	path := string(bytes.TrimSpace(out))
	r.resolved[name] = path
	return path, nil
}

// Run implements Runner.
func (r *Real) Run(name string, args ...string) (int, error) {
	path, err := r.Resolve(name)
	if err != nil {
		return 1, err
	}
	cmd := exec.Command(path, args...) // #nosec G204 -- path resolved via `go tool -n`/PATH by go-canon itself
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("run %s: %w", name, err)
	}
	return 0, nil
}

// Output implements Runner.
func (r *Real) Output(name string, args ...string) ([]byte, error) {
	path, err := r.Resolve(name)
	if err != nil {
		return nil, err
	}
	return capture(path, name, args...)
}

// capture runs path with args, capturing stdout; stderr is folded into
// the error.
func capture(path, name string, args ...string) ([]byte, error) {
	var errBuf bytes.Buffer
	cmd := exec.Command(path, args...) // #nosec G204 -- path resolved via `go tool -n`/PATH by go-canon itself
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return out, fmt.Errorf("%s exited with code %d: %s", name, exitErr.ExitCode(), bytes.TrimSpace(errBuf.Bytes()))
		}
		return out, fmt.Errorf("run %s: %w", name, err)
	}
	return out, nil
}
