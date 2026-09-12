// Package app implements the go-canon command line: the shared lint,
// format, test, doctor, and migrate pipelines.
package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/SynthLuvlr/go-canon/internal/config"
	"github.com/SynthLuvlr/go-canon/internal/execx"
	"github.com/SynthLuvlr/go-canon/internal/module"
	"github.com/SynthLuvlr/go-canon/internal/tools"
)

const usage = `go-canon — shared Go toolchain gates and presets (` + tools.CanonVersion + `)

Usage:
  go-canon lint [flags] [paths...]    static pipeline: build, lint, idioms,
                                      tidy, vulncheck, markdown
  go-canon format [flags] [paths...]  apply modernize, gofumpt/gci, tidy,
                                      pandoc
  go-canon test                       go test -race + coverage gate
  go-canon check [flags]              lint, then test
  go-canon doctor                     environment diagnostics
  go-canon migrate [flags]            adopt go-canon in an existing repo

Flags:
  --fast         skip the slow gates (govulncheck, dupl, markdown)
  --keep-config  keep the generated .go-canon/ effective config
  --dry-run      (migrate) print the planned changes only
  --dir PATH     (migrate) target repository, default "."

Every gate step's exit code propagates (fail-fast).
`

// errUsage marks flag/usage errors, which exit 2 instead of 1.
var errUsage = errors.New("usage error")

// env is the resolved per-invocation environment shared by commands.
type env struct {
	root    string
	cfg     config.Config
	cfgPath string
	r       execx.Runner
	stdout  io.Writer
	stderr  io.Writer
	keep    bool
	fast    bool
	paths   []string
}

// Run executes the CLI — argv without the program name, r for external
// tools, stdout/stderr for reporting — and returns the exit code.
func Run(argv []string, r execx.Runner, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, args := argv[0], argv[1:]
	switch cmd {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "lint":
		return withEnv(args, r, stdout, stderr, runLint)
	case "format":
		return withEnv(args, r, stdout, stderr, runFormat)
	case "test":
		return withEnv(args, r, stdout, stderr, runTest)
	case "check":
		return withEnv(args, r, stdout, stderr, runCheck)
	case "doctor":
		return runDoctor(args, r, stdout)
	case "migrate":
		return runMigrate(args, r, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "go-canon: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

// withEnv sets up the module environment and runs fn with it.
func withEnv(args []string, r execx.Runner, stdout, stderr io.Writer, fn func(*env) int) int {
	e, err := setup(args, r, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "go-canon: %v\n", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	return fn(e)
}

// setup parses shared flags, finds the module root, and loads config.
func setup(args []string, r execx.Runner, stdout, stderr io.Writer) (*env, error) {
	flags := flag.NewFlagSet("go-canon", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fast := flags.Bool("fast", false, "skip the slow gates")
	keep := flags.Bool("keep-config", false, "keep the generated effective config")
	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("%w: %w", errUsage, err)
	}
	root, err := module.Root()
	if err != nil {
		return nil, fmt.Errorf("find module root: %w", err)
	}
	cfg, cfgPath, err := config.Load(root)
	if err != nil {
		return nil, fmt.Errorf("load go-canon config: %w", err)
	}
	if err := os.Chdir(root); err != nil {
		return nil, fmt.Errorf("chdir %s: %w", root, err)
	}
	return &env{
		root:    root,
		cfg:     cfg,
		cfgPath: cfgPath,
		r:       r,
		stdout:  stdout,
		stderr:  stderr,
		keep:    *keep,
		fast:    *fast,
		paths:   flags.Args(),
	}, nil
}

// step is one fail-fast pipeline step.
type step struct {
	title string
	run   func() (int, error)
}

// run executes the steps in order, stopping at the first failure; each
// step's exit code propagates.
func (e *env) run(steps []step) int {
	for i, s := range steps {
		fmt.Fprintf(e.stdout, "==> [%d/%d] %s\n", i+1, len(steps), s.title)
		code, err := s.run()
		if err != nil {
			fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
			return 1
		}
		if code != 0 {
			fmt.Fprintf(e.stdout, "==> %s failed (exit %d)\n", s.title, code)
			return code
		}
	}
	return 0
}

// cmd builds a step that runs one external tool.
func (e *env) cmd(title, tool string, argv ...string) step {
	return step{title: title, run: func() (int, error) { return e.r.Run(tool, argv...) }}
}

// targetPaths returns the user's path args, or ./... when none were given.
func (e *env) targetPaths() []string {
	if len(e.paths) == 0 {
		return []string{"./..."}
	}
	return e.paths
}
