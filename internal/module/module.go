// Package module locates the root of the enclosing Go module.
package module

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Root returns the absolute path of the directory containing go.mod,
// walking up from the process working directory.
func Root() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return RootFrom(wd)
}

// RootFrom walks up from dir until it finds a directory containing
// go.mod.
func RootFrom(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("empty start directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("absolutize %s: %w", dir, err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("stat go.mod in %s: %w", abs, err)
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no go.mod found at or above %s", dir)
		}
		abs = parent
	}
}
