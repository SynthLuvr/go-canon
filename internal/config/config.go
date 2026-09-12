// Package config loads per-repo `go-canon.toml` deltas over the built-in
// presets.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Names are the config file names recognized at the module root, in
// precedence order.
var Names = []string{"go-canon.toml", "gocanon.toml"}

// Test configures the coverage gate.
type Test struct {
	// CoverageThreshold is the minimum total statement coverage percent.
	CoverageThreshold float64 `toml:"coverage-threshold"`
}

// Dupl is sugar for the golangci-lint dupl token threshold.
type Dupl struct {
	// Tokens overrides the dupl threshold; 0 keeps the preset default.
	Tokens int `toml:"tokens"`
}

// Markdown configures the pandoc markdown gate.
type Markdown struct {
	// Exclude lists glob patterns, relative to the module root with `/`
	// separators, skipped by the markdown gate.
	Exclude []string `toml:"exclude"`
}

// Config is the full go-canon.toml surface.
type Config struct {
	// Golangci holds deep-merge deltas over the embedded golangci-lint
	// preset: maps merge recursively, lists and scalars replace.
	Golangci map[string]any `toml:"golangci"`
	// Test configures the coverage gate.
	Test Test `toml:"test"`
	// Dupl tunes the duplication gate.
	Dupl Dupl `toml:"dupl"`
	// Markdown tunes the markdown gate.
	Markdown Markdown `toml:"markdown"`
}

// Default returns the preset defaults with no local deltas.
func Default() Config {
	return Config{Test: Test{CoverageThreshold: 80}}
}

// Load reads go-canon.toml (or gocanon.toml) from root, layering it over
// Default. The returned path is empty when no config file exists.
func Load(root string) (Config, string, error) {
	cfg := Default()
	for _, name := range Names {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path) // #nosec G304 -- fixed name under the module root
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Config{}, "", fmt.Errorf("read %s: %w", path, err)
		}
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return Config{}, "", fmt.Errorf("parse %s: %w", path, err)
		}
		if cfg.Test.CoverageThreshold == 0 {
			cfg.Test.CoverageThreshold = Default().Test.CoverageThreshold
		}
		return cfg, path, nil
	}
	return cfg, "", nil
}

// Delta returns the effective golangci-lint config delta, folding the
// dupl-token sugar into the [golangci] tree.
func (c Config) Delta() map[string]any {
	delta := map[string]any{}
	maps.Copy(delta, c.Golangci)
	if c.Dupl.Tokens <= 0 {
		return delta
	}
	node := delta
	for _, key := range []string{"linters", "settings", "dupl"} {
		child, ok := node[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[key] = child
		}
		node = child
	}
	node["threshold"] = c.Dupl.Tokens
	return delta
}
