package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	got := Default()
	if got.Test.CoverageThreshold != 80 {
		t.Errorf("default coverage threshold = %v, want 80", got.Test.CoverageThreshold)
	}
	if got.Dupl.Tokens != 0 {
		t.Errorf("default dupl tokens = %v, want 0", got.Dupl.Tokens)
	}
	if len(got.Golangci) != 0 {
		t.Errorf("default golangci delta = %v, want empty", got.Golangci)
	}
}

func TestLoadWithoutFile(t *testing.T) {
	cfg, path, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Errorf("path = %q, want empty", path)
	}
	if cfg.Test.CoverageThreshold != 80 {
		t.Errorf("coverage threshold = %v, want default 80", cfg.Test.CoverageThreshold)
	}
}

const deltaTOML = `
[golangci.linters.settings.dupl]
threshold = 200

[test]
coverage-threshold = 60.5

[dupl]
tokens = 222

[markdown]
exclude = ["docs/*.md"]
`

func TestLoadGoCanonToml(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go-canon.toml"), []byte(deltaTOML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, path, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "go-canon.toml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if cfg.Test.CoverageThreshold != 60.5 {
		t.Errorf("coverage threshold = %v, want 60.5", cfg.Test.CoverageThreshold)
	}
	if cfg.Dupl.Tokens != 222 {
		t.Errorf("dupl tokens = %v, want 222", cfg.Dupl.Tokens)
	}
	wantExclude := []string{"docs/*.md"}
	if len(cfg.Markdown.Exclude) != 1 || cfg.Markdown.Exclude[0] != wantExclude[0] {
		t.Errorf("markdown exclude = %v, want %v", cfg.Markdown.Exclude, wantExclude)
	}
	settings, ok := cfg.Golangci["linters"].(map[string]any)
	if !ok {
		t.Fatalf("golangci.linters delta missing: %v", cfg.Golangci)
	}
	if got := settings["settings"]; got == nil {
		t.Errorf("golangci.linters.settings delta missing: %v", settings)
	}
}

func TestLoadGocanonFallbackName(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gocanon.toml"), []byte("[test]\ncoverage-threshold = 90\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, path, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "gocanon.toml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if cfg.Test.CoverageThreshold != 90 {
		t.Errorf("coverage threshold = %v, want 90", cfg.Test.CoverageThreshold)
	}
}

func TestLoadZeroThresholdKeepsDefault(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go-canon.toml"), []byte("[test]\ncoverage-threshold = 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Test.CoverageThreshold != 80 {
		t.Errorf("coverage threshold = %v, want default 80", cfg.Test.CoverageThreshold)
	}
}

func TestLoadMalformed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go-canon.toml"), []byte("= not toml ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(root); err == nil {
		t.Error("Load(malformed) succeeded, want error")
	}
}

func TestDeltaFoldsDuplSugar(t *testing.T) {
	cfg := Default()
	cfg.Dupl.Tokens = 175
	delta := cfg.Delta()
	linters, ok := delta["linters"].(map[string]any)
	if !ok {
		t.Fatalf("delta.linters = %v, want a map", delta)
	}
	settings, ok := linters["settings"].(map[string]any)
	if !ok {
		t.Fatalf("delta.linters.settings = %v, want a map", linters)
	}
	dupl, ok := settings["dupl"].(map[string]any)
	if !ok {
		t.Fatalf("delta.linters.settings.dupl = %v, want a map", settings)
	}
	if dupl["threshold"] != 175 {
		t.Errorf("dupl threshold = %v, want 175", dupl["threshold"])
	}
}

func TestDeltaWithoutSugarIsEmpty(t *testing.T) {
	cfg := Default()
	if delta := cfg.Delta(); len(delta) != 0 {
		t.Errorf("Delta() = %v, want empty", delta)
	}
}

func TestDeltaPreservesGolangciTree(t *testing.T) {
	cfg := Default()
	cfg.Golangci = map[string]any{"version": "2"}
	cfg.Dupl.Tokens = 100
	delta := cfg.Delta()
	if delta["version"] != "2" {
		t.Errorf("Delta() lost golangci tree: %v", delta)
	}
	if _, ok := delta["linters"]; !ok {
		t.Errorf("Delta() missing folded dupl tree: %v", delta)
	}
}
