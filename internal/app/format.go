package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SynthLuvlr/go-canon/internal/markdown"
)

// runFormat applies every formatter in order: modernize -fix,
// golangci-lint fmt (gofumpt + gci from the effective config), go mod
// tidy, and the pandoc markdown rewrite.
func runFormat(e *env) int {
	cfgPath, cleanup, err := e.writeEffectiveConfig()
	if err != nil {
		fmt.Fprintf(e.stderr, "go-canon: %v\n", err)
		return 1
	}
	defer cleanup()

	fmtArgs := append([]string{"fmt", "--config", cfgPath}, e.paths...)
	return e.run([]step{
		e.fixIdioms(e.targetPaths()),
		e.cmd("golangci-lint fmt", "golangci-lint", fmtArgs...),
		e.cmd("go mod tidy", "go", "mod", "tidy"),
		e.formatMarkdown(),
	})
}

// fixIdioms builds the modernize -fix step, tolerating findings it
// cannot auto-fix (exit 3): lint reports those afterwards.
func (e *env) fixIdioms(paths []string) step {
	return step{
		title: "modernize -fix",
		run: func() (int, error) {
			code, err := e.r.Run("modernize", append([]string{"-fix"}, paths...)...)
			if err != nil {
				return 1, fmt.Errorf("run modernize -fix: %w", err)
			}
			if code == 3 {
				fmt.Fprintln(e.stdout, "go-canon: modernize left findings it could not auto-fix; lint will report them")
				return 0, nil
			}
			return code, nil
		},
	}
}

// formatMarkdown builds the pandoc rewrite step for the markdown files
// under the user's path args.
func (e *env) formatMarkdown() step {
	return step{
		title: "pandoc markdown",
		run: func() (int, error) {
			files, err := markdown.Files(e.root, e.cfg.Markdown.Exclude)
			if err != nil {
				return 1, fmt.Errorf("collect markdown files: %w", err)
			}
			files = e.underPaths(files)
			if err := markdown.Format(e.r, files); err != nil {
				return 1, fmt.Errorf("normalize markdown: %w", err)
			}
			fmt.Fprintf(e.stdout, "    normalized %d markdown files\n", len(files))
			return 0, nil
		},
	}
}

// underPaths filters files to those below any user-provided path arg;
// without path args every file is kept.
func (e *env) underPaths(files []string) []string {
	if len(e.paths) == 0 {
		return files
	}
	var prefixes []string
	for _, p := range e.paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		prefixes = append(prefixes, abs)
	}
	var kept []string
	for _, file := range files {
		for _, prefix := range prefixes {
			if rel, err := filepath.Rel(prefix, file); err == nil && !strings.HasPrefix(rel, "..") {
				kept = append(kept, file)
				break
			}
		}
	}
	return kept
}
