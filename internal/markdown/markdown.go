// Package markdown enforces pandoc-normalized GFM (LF) across the
// module's markdown files.
package markdown

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/SynthLuvr/go-canon/internal/execx"
	"github.com/SynthLuvr/go-canon/internal/tools"
)

// skipDirs are never walked for markdown files.
var skipDirs = map[string]bool{
	".git":         true,
	".go-canon":    true,
	"node_modules": true,
	"testdata":     true,
	"vendor":       true,
}

// Files returns the markdown files under root, sorted lexically, skipping
// built-in directories and the user's exclude globs (relative to root,
// `/`-separated).
func Files(root string, exclude []string) ([]string, error) {
	var files []string
	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		if excluded(filepath.ToSlash(rel), exclude) {
			return nil
		}
		files = append(files, path)
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return files, nil
}

// excluded reports whether rel matches any exclude glob.
func excluded(rel string, exclude []string) bool {
	for _, pattern := range exclude {
		matched, err := filepath.Match(pattern, rel)
		if err != nil {
			// A malformed pattern matches nothing rather than failing the walk.
			return false
		}
		if matched {
			return true
		}
	}
	return false
}

// Check returns the files that are not byte-identical to their pandoc GFM
// rendering.
func Check(r execx.Runner, files []string) ([]string, error) {
	var drifted []string
	for _, file := range files {
		want, err := rendered(r, file)
		if err != nil {
			return nil, err
		}
		got, err := os.ReadFile(file) // #nosec G304 -- paths come from the module tree walk
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		if !bytes.Equal(got, want) {
			drifted = append(drifted, file)
		}
	}
	return drifted, nil
}

// Format rewrites each file in place to its pandoc GFM rendering,
// preserving file permissions.
func Format(r execx.Runner, files []string) error {
	for _, file := range files {
		want, err := rendered(r, file)
		if err != nil {
			return err
		}
		info, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("stat %s: %w", file, err)
		}
		if err := os.WriteFile(file, want, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", file, err)
		}
	}
	return nil
}

// rendered returns the pandoc GFM (LF) form of file.
func rendered(r execx.Runner, file string) ([]byte, error) {
	out, err := r.Output("pandoc", "--eol=lf", "-t", "gfm", file)
	if err != nil {
		return nil, fmt.Errorf("pandoc %s — is pandoc %s+ installed and on PATH?: %w", file, tools.PandocMin, err)
	}
	return out, nil
}
