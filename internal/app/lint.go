package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/SynthLuvr/gocanon/internal/golangci"
	"github.com/SynthLuvr/gocanon/internal/markdown"
)

// runLint runs the fail-fast static pipeline: go build, golangci-lint
// (from the merged effective config), modernize, go mod tidy -diff,
// then govulncheck and the pandoc markdown check unless --fast, which
// also disables the dupl gate inside golangci-lint.
func runLint(e *env) int {
	cfgPath, cleanup, err := e.writeEffectiveConfig()
	if err != nil {
		fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
		return 1
	}
	defer cleanup()

	paths := e.targetPaths()
	golangciArgs := []string{"run", "--config", cfgPath}
	if e.fast {
		golangciArgs = append(golangciArgs, "--disable=dupl")
	}
	golangciArgs = append(golangciArgs, paths...)

	steps := []step{
		e.cmd("go build", "go", append([]string{"build"}, paths...)...),
		e.cmd("golangci-lint", "golangci-lint", golangciArgs...),
		e.cmd("modernize", "modernize", paths...),
		e.cmd("go mod tidy -diff", "go", "mod", "tidy", "-diff"),
	}
	if !e.fast {
		steps = append(steps, e.cmd("govulncheck", "govulncheck", paths...), e.markdownCheck())
	}
	return e.run(steps)
}

// markdownCheck builds the pandoc GFM gate step.
func (e *env) markdownCheck() step {
	return step{
		title: "pandoc markdown",
		run: func() (int, error) {
			files, err := markdown.Files(e.root, e.cfg.Markdown.Exclude)
			if err != nil {
				return 1, fmt.Errorf("collect markdown files: %w", err)
			}
			drifted, err := markdown.Check(e.r, files)
			if err != nil {
				return 1, fmt.Errorf("check markdown formatting: %w", err)
			}
			if len(drifted) == 0 {
				return 0, nil
			}
			for _, file := range drifted {
				fmt.Fprintf(e.stdout, "    %s is not pandoc-GFM\n", file)
			}
			fmt.Fprintln(e.stderr, "go-canon: run `go-canon format` to normalize the files above")
			return 1, nil
		},
	}
}

// writeEffectiveConfig materializes the merged golangci-lint config under
// .go-canon/ and returns its path plus a cleanup that removes the scratch
// dir (kept with --keep-config).
func (e *env) writeEffectiveConfig() (string, func(), error) {
	data, err := golangci.Effective(e.cfg.Delta())
	if err != nil {
		return "", nil, fmt.Errorf("build effective golangci config: %w", err)
	}
	dir := filepath.Join(e.root, golangci.DirName)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, fmt.Errorf("create %s: %w", dir, err)
	}
	path := filepath.Join(dir, golangci.FileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", nil, fmt.Errorf("write %s: %w", path, err)
	}
	cleanup := func() {
		if e.keep {
			fmt.Fprintf(e.stdout, "go-canon: keeping %s\n", path)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(e.stderr, "go-canon: remove %s: %v\n", dir, err)
		}
	}
	return path, cleanup, nil
}
