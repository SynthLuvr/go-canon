package app

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/SynthLuvlr/gocanon/internal/execx"
	"github.com/SynthLuvlr/gocanon/internal/tools"
)

//go:embed templates/Taskfile.yml
var taskfileTemplate []byte

//go:embed templates/go-canon.toml
var canonTemplate []byte

// gitignoreLines are the go-canon artifacts a repo must ignore.
var gitignoreLines = []string{".go-canon/", "cover.out"}

// gitattributesLine normalizes line endings to LF, as in the sibling
// templates.
const gitattributesLine = "* text=auto eol=lf"

// goDirective matches the `go 1.24` line of a go.mod.
var goDirective = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)

// action is one migration change: described for humans, executable for
// real. A nil apply marks a report-only entry, such as a warning.
type action struct {
	desc  string
	apply func() error
}

// runMigrate adopts go-canon in an existing repository: it writes the
// go.mod tool block entries, Taskfile.yml, go-canon.toml, and the
// housekeeping files, then tidies the module. --dry-run prints the plan.
func runMigrate(args []string, r execx.Runner, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dryRun := flags.Bool("dry-run", false, "print the planned changes only")
	dir := flags.String("dir", ".", "target repository")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	root, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(stderr, "go-canon: resolve %s: %v\n", *dir, err)
		return 1
	}
	gomodPath := filepath.Join(root, "go.mod")
	gomod, err := os.ReadFile(gomodPath) // #nosec G304 -- caller-supplied repository root
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "go-canon: %s has no go.mod — migrate targets Go modules\n", root)
		} else {
			fmt.Fprintf(stderr, "go-canon: read %s: %v\n", gomodPath, err)
		}
		return 1
	}
	actions := planMigrate(r, root, string(gomod))

	if *dryRun {
		for _, a := range actions {
			fmt.Fprintf(stdout, "[dry-run] %s\n", a.desc)
		}
		fmt.Fprintf(stdout, "[dry-run] %d planned change(s)\n", len(actions))
		return 0
	}
	// Child tools run from the target repo, but the chdir must not
	// leak to the caller: Windows cannot delete a directory that is
	// the process cwd, which breaks t.TempDir cleanup in tests.
	origWd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "go-canon: get working directory: %v\n", err)
		return 1
	}
	if err := os.Chdir(root); err != nil {
		fmt.Fprintf(stderr, "go-canon: chdir %s: %v\n", root, err)
		return 1
	}
	defer func() {
		if err := os.Chdir(origWd); err != nil {
			fmt.Fprintf(stderr, "go-canon: restore working directory: %v\n", err)
		}
	}()
	for _, a := range actions {
		fmt.Fprintf(stdout, "go-canon: %s\n", a.desc)
		if a.apply == nil {
			continue
		}
		if err := a.apply(); err != nil {
			fmt.Fprintf(stderr, "go-canon: %v\n", err)
			return 1
		}
	}
	return 0
}

// planMigrate builds the ordered action list for the target repo.
func planMigrate(r execx.Runner, root, gomod string) []action {
	var actions []action

	if m := goDirective.FindStringSubmatch(gomod); m != nil && !atLeast(m[1], tools.GoFloor) {
		actions = append(actions, action{
			desc: "go mod edit -go=" + tools.GoFloor + " (go-canon's pinned toolchain needs go >= " + tools.GoFloor + ")",
			apply: func() error {
				return runGo(r, "mod", "edit", "-go="+tools.GoFloor)
			},
		})
	}

	release := tools.CanonRelease()
	pinned := slices.Concat(tools.GoTools, []tools.Tool{tools.TaskTool})
	if release != "" {
		pinned = append(pinned, tools.CanonTool(release))
	}
	for _, t := range pinned {
		if strings.Contains(gomod, t.Pkg) {
			continue
		}
		ref := t.Pkg + "@" + t.Version
		actions = append(actions, action{
			desc: "go get -tool " + ref,
			apply: func() error {
				return runGo(r, "get", "-tool", ref)
			},
		})
	}

	if release == "" && !strings.Contains(gomod, tools.CanonPkg) {
		actions = append(actions, action{
			desc: "warning: development build — pin go-canon manually: go get -tool " + tools.CanonPkg + "@vX.Y.Z",
		})
	}

	actions = append(actions,
		fileAction(root, "Taskfile.yml", taskfileTemplate),
		fileAction(root, "go-canon.toml", canonTemplate),
		fileAction(root, ".go-version", []byte(tools.GoToolchain+"\n")),
		appendLinesAction(root, ".gitignore", gitignoreLines),
		appendLinesAction(root, ".gitattributes", []string{gitattributesLine}),
		action{
			desc: "go mod tidy",
			apply: func() error {
				return runGo(r, "mod", "tidy")
			},
		},
	)
	return actions
}

// runGo runs a go subcommand through the runner, converting non-zero
// exits to errors.
func runGo(r execx.Runner, args ...string) error {
	code, err := r.Run("go", args...)
	if err != nil {
		return fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	if code != 0 {
		return fmt.Errorf("go %s: exit code %d", strings.Join(args, " "), code)
	}
	return nil
}

// fileAction writes content to path when the file does not exist yet.
func fileAction(root, name string, content []byte) action {
	path := filepath.Join(root, name)
	return action{
		desc: "write " + name,
		apply: func() error {
			data, err := os.ReadFile(path) // #nosec G304 -- fixed name under the target repo
			if err == nil && len(data) > 0 {
				return nil
			}
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if err := os.WriteFile(path, content, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			return nil
		},
	}
}

// appendLinesAction appends the missing lines to path, creating it if
// needed.
func appendLinesAction(root, name string, lines []string) action {
	path := filepath.Join(root, name)
	return action{
		desc: "append " + strings.Join(lines, ", ") + " to " + name,
		apply: func() error {
			existing, err := os.ReadFile(path) // #nosec G304 -- fixed name under the target repo
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var missing []string
			for _, line := range lines {
				if !strings.Contains(string(existing), line) {
					missing = append(missing, line)
				}
			}
			if len(missing) == 0 {
				return nil
			}
			content := existing
			if len(content) > 0 && content[len(content)-1] != '\n' {
				content = append(content, '\n')
			}
			content = append(content, []byte(strings.Join(missing, "\n")+"\n")...)
			if err := os.WriteFile(path, content, 0o600); err != nil { // #nosec G703 -- path = operator-supplied repo root + fixed file names
				return fmt.Errorf("write %s: %w", path, err)
			}
			return nil
		},
	}
}
