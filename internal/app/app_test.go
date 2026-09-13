package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SynthLuvr/gocanon/internal/testrunner"
)

// writeModule creates a scratch module in a temp dir and chdirs into it.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	return root
}

const scratchGoMod = "module example.com/scratch\n\ngo 1.27\n"

// pandocEcho fakes pandoc: renders the named file verbatim.
func pandocEcho(name string, args []string) ([]byte, error) {
	return os.ReadFile(args[len(args)-1]) // #nosec G304 -- test-provided fixture path
}

func TestRunUsage(t *testing.T) {
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run(nil, fake, &out, &errOut); code != 2 {
		t.Errorf("Run(no args) = %d, want 2", code)
	}
	if code := Run([]string{"frobnicate"}, fake, &out, &errOut); code != 2 {
		t.Errorf("Run(unknown) = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("stderr = %q, want unknown-command message", errOut.String())
	}
	if code := Run([]string{"--help"}, fake, &out, &errOut); code != 0 {
		t.Errorf("Run(--help) = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("stdout = %q, want usage", out.String())
	}
}

func TestRunVersion(t *testing.T) {
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"--version"}, fake, &out, &errOut); code != 0 {
		t.Errorf("Run(--version) = %d, want 0", code)
	}
	version, ok := strings.CutPrefix(out.String(), "go-canon ")
	if !ok || strings.TrimSpace(version) == "" {
		t.Errorf("stdout = %q, want a go-canon version line", out.String())
	}
}

func TestRunBadFlag(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"lint", "--nope"}, fake, &out, &errOut); code != 2 {
		t.Errorf("Run(lint --nope) = %d, want 2", code)
	}
}

func TestLintPipelineOrder(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
		"index.go":  "package scratch\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"lint"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("lint exit = %d, stderr:\n%s", code, errOut.String())
	}
	cfgPath := filepath.Join(root, ".go-canon", "golangci.yml")
	want := []string{
		"go build ./...",
		"golangci-lint run --config " + cfgPath + " ./...",
		"modernize ./...",
		"go mod tidy -diff",
		"govulncheck ./...",
		"pandoc --eol=lf -t gfm " + filepath.Join(root, "README.md"),
	}
	if len(fake.Commands) != len(want) {
		t.Fatalf("commands =\n%s\nwant %d entries", strings.Join(fake.Commands, "\n"), len(want))
	}
	for i := range want {
		if fake.Commands[i] != want[i] {
			t.Errorf("command[%d] = %q, want %q", i, fake.Commands[i], want[i])
		}
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("effective config scratch dir not cleaned up")
	}
}

func TestLintFastSkipsSlowGates(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"lint", "--fast"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("lint --fast exit = %d, stderr:\n%s", code, errOut.String())
	}
	if fake.Has("govulncheck") {
		t.Error("--fast still ran govulncheck")
	}
	if fake.Has("pandoc") {
		t.Error("--fast still ran the markdown gate")
	}
	for _, cmd := range fake.Commands {
		if strings.HasPrefix(cmd, "golangci-lint") && !strings.Contains(cmd, "--disable=dupl") {
			t.Errorf("golangci-lint kept dupl enabled: %q", cmd)
		}
	}
}

func TestLintFailFastPropagatesExitCode(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{
		RunFn: func(name string, args []string) (int, error) {
			if name == "golangci-lint" {
				return 7, nil
			}
			return 0, nil
		},
		OutputFn: pandocEcho,
	}
	var out, errOut strings.Builder
	if code := Run([]string{"lint"}, fake, &out, &errOut); code != 7 {
		t.Fatalf("lint exit = %d, want 7 (propagated)", code)
	}
	if fake.Has("modernize") {
		t.Error("pipeline continued past a failing step")
	}
}

func TestLintMarkdownDriftFails(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# messy\n",
	})
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			return []byte("# canonical\n"), nil
		},
	}
	var out, errOut strings.Builder
	if code := Run([]string{"lint"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("lint exit = %d, want 1 for markdown drift", code)
	}
	if !strings.Contains(errOut.String(), "go-canon format") {
		t.Errorf("stderr = %q, want format guidance", errOut.String())
	}
}

func TestLintKeepConfig(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"lint", "--keep-config"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("lint --keep-config exit = %d, stderr:\n%s", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".go-canon", "golangci.yml")); err != nil {
		t.Errorf("effective config not kept: %v", err)
	}
}

func TestLintConfigDeltaReachesEffectiveConfig(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":        scratchGoMod,
		"README.md":     "# scratch\n",
		"go-canon.toml": "[dupl]\ntokens = 250\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"lint", "--keep-config"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("lint exit = %d, stderr:\n%s", code, errOut.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".go-canon", "golangci.yml")) // #nosec G304 -- scratch config written by the test
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "threshold: 250") {
		t.Errorf("effective config missing dupl delta:\n%s", data)
	}
}

func TestLintBadConfigFails(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":        scratchGoMod,
		"go-canon.toml": "= not toml",
	})
	fake := &testrunner.Fake{}
	var out, errOut strings.Builder
	if code := Run([]string{"lint"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("lint exit = %d, want 1 for bad config", code)
	}
	if !strings.Contains(errOut.String(), "parse") {
		t.Errorf("stderr = %q, want parse error", errOut.String())
	}
}

func TestFormatPipelineOrder(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# messy\n",
		"index.go":  "package scratch\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"format"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("format exit = %d, stderr:\n%s", code, errOut.String())
	}
	cfgPath := filepath.Join(root, ".go-canon", "golangci.yml")
	want := []string{
		"modernize -fix ./...",
		"golangci-lint fmt --config " + cfgPath,
		"go mod tidy",
		"pandoc --eol=lf -t gfm " + filepath.Join(root, "README.md"),
	}
	if len(fake.Commands) != len(want) {
		t.Fatalf("commands =\n%s\nwant %d entries", strings.Join(fake.Commands, "\n"), len(want))
	}
	for i := range want {
		if fake.Commands[i] != want[i] {
			t.Errorf("command[%d] = %q, want %q", i, fake.Commands[i], want[i])
		}
	}
}

func TestFormatToleratesModernizeFindings(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{
		RunFn: func(name string, args []string) (int, error) {
			if name == "modernize" {
				return 3, nil // findings left unfixed
			}
			return 0, nil
		},
		OutputFn: pandocEcho,
	}
	var out, errOut strings.Builder
	if code := Run([]string{"format"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("format exit = %d, want 0 (exit 3 is tolerated)", code)
	}
	if !strings.Contains(out.String(), "could not auto-fix") {
		t.Errorf("stdout = %q, want an unfixed-findings warning", out.String())
	}
}

func TestFormatPropagatesModernizeFailure(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{
		RunFn: func(name string, args []string) (int, error) {
			if name == "modernize" {
				return 2, nil // hard failure, not findings
			}
			return 0, nil
		},
		OutputFn: pandocEcho,
	}
	var out, errOut strings.Builder
	if code := Run([]string{"format"}, fake, &out, &errOut); code != 2 {
		t.Fatalf("format exit = %d, want 2 (propagated)", code)
	}
	if fake.Has("golangci-lint") {
		t.Error("pipeline continued past a failing modernize")
	}
}

func TestFormatPathArgsReachToolsAndMarkdown(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":        scratchGoMod,
		"README.md":     "# root\n",
		"docs/guide.md": "# guide\n",
	})
	fake := &testrunner.Fake{OutputFn: pandocEcho}
	var out, errOut strings.Builder
	if code := Run([]string{"format", "./docs"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("format ./docs exit = %d, stderr:\n%s", code, errOut.String())
	}
	for _, cmd := range fake.Commands {
		if strings.HasPrefix(cmd, "golangci-lint") && !strings.Contains(cmd, "./docs") {
			t.Errorf("path arg not forwarded to golangci-lint: %q", cmd)
		}
	}
	if fake.Has("pandoc --eol=lf -t gfm " + filepath.Join(root, "README.md")) {
		t.Error("markdown outside the path filter was rewritten")
	}
	if !fake.Has("pandoc --eol=lf -t gfm " + filepath.Join(root, "docs", "guide.md")) {
		t.Error("markdown under the path filter was not rewritten")
	}
}

func TestTestCommandCoverageGate(t *testing.T) {
	tests := []struct {
		name     string
		coverOut string
		wantCode int
	}{
		{name: "above threshold", coverOut: "total:\t(statements)\t92.0%\n", wantCode: 0},
		{name: "at threshold", coverOut: "total:\t(statements)\t80.0%\n", wantCode: 0},
		{name: "below threshold", coverOut: "total:\t(statements)\t61.5%\n", wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeModule(t, map[string]string{"go.mod": scratchGoMod})
			fake := &testrunner.Fake{
				OutputFn: func(name string, args []string) ([]byte, error) {
					if name == "go" {
						return []byte(tt.coverOut), nil
					}
					return nil, fmt.Errorf("unexpected output call: %s %v", name, args)
				},
			}
			var out, errOut strings.Builder
			if code := Run([]string{"test"}, fake, &out, &errOut); code != tt.wantCode {
				t.Fatalf("test exit = %d, want %d\nstderr:\n%s", code, tt.wantCode, errOut.String())
			}
			if !fake.Has("go test ./... -race -covermode=atomic -coverprofile=cover.out -coverpkg=./...") {
				t.Errorf("go test invocation wrong: %v", fake.Commands)
			}
			if !fake.Has("go tool cover -func=cover.out") {
				t.Errorf("go tool cover -func not run: %v", fake.Commands)
			}
		})
	}
}

func TestTestCommandPropagatesGoTestFailure(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	fake := &testrunner.Fake{
		RunFn: func(name string, args []string) (int, error) {
			if name == "go" {
				return 5, nil
			}
			return 0, nil
		},
	}
	var out, errOut strings.Builder
	if code := Run([]string{"test"}, fake, &out, &errOut); code != 5 {
		t.Fatalf("test exit = %d, want 5 (propagated)", code)
	}
	if fake.Has("go tool cover") {
		t.Error("coverage gate ran despite failing tests")
	}
}

func TestTestCommandThresholdFromConfig(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":        scratchGoMod,
		"go-canon.toml": "[test]\ncoverage-threshold = 60\n",
	})
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			return []byte("total:\t(statements)\t61.5%\n"), nil
		},
	}
	var out, errOut strings.Builder
	if code := Run([]string{"test"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("test exit = %d, want 0 (61.5%% >= 60%% threshold from config)", code)
	}
}

func TestCheckRunsLintThenTest(t *testing.T) {
	writeModule(t, map[string]string{
		"go.mod":    scratchGoMod,
		"README.md": "# scratch\n",
	})
	fake := &testrunner.Fake{
		OutputFn: func(name string, args []string) ([]byte, error) {
			if name == "go" {
				return []byte("total:\t(statements)\t100.0%\n"), nil
			}
			return pandocEcho(name, args)
		},
	}
	var out, errOut strings.Builder
	if code := Run([]string{"check"}, fake, &out, &errOut); code != 0 {
		t.Fatalf("check exit = %d, stderr:\n%s", code, errOut.String())
	}
	if !fake.Has("go build") || !fake.Has("go test") {
		t.Errorf("check did not run both pipelines: %v", fake.Commands)
	}
}

func TestCheckStopsAtLintFailure(t *testing.T) {
	writeModule(t, map[string]string{"go.mod": scratchGoMod})
	fake := &testrunner.Fake{
		RunFn: func(name string, args []string) (int, error) {
			if name == "go" && len(args) > 0 && args[0] == "build" {
				return 1, nil
			}
			return 0, nil
		},
	}
	var out, errOut strings.Builder
	if code := Run([]string{"check"}, fake, &out, &errOut); code != 1 {
		t.Fatalf("check exit = %d, want 1 (lint failed)", code)
	}
	if fake.Has("go test") {
		t.Error("test ran despite lint failure")
	}
}
