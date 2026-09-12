// Package golangci produces the effective golangci-lint configuration by
// deep-merging local go-canon.toml deltas over the embedded preset.
//
// golangci-lint v2 has no config inheritance, so go-canon writes the merged
// result to .go-canon/golangci.yml at invocation time and passes it via
// --config (canonist's generated-effective-config pattern).
package golangci

import (
	"bytes"
	_ "embed"
	"fmt"
	"maps"

	"go.yaml.in/yaml/v3"
)

// DirName is the scratch directory for generated effective configs.
const DirName = ".go-canon"

// FileName is the effective config file name inside DirName.
const FileName = "golangci.yml"

// preset is the canonical golangci-lint configuration shipped by go-canon.
// Consumers never edit this file: deltas go in go-canon.toml [golangci].
//
//go:embed preset.yml
var preset []byte

// Effective merges delta over the embedded preset and returns the
// serialized effective configuration.
func Effective(delta map[string]any) ([]byte, error) {
	var base map[string]any
	if err := yaml.Unmarshal(preset, &base); err != nil {
		return nil, fmt.Errorf("parse embedded preset: %w", err)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(Merge(base, delta)); err != nil {
		return nil, fmt.Errorf("encode effective config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// Merge deep-merges delta into base and returns the result without
// mutating either input: maps merge recursively, and every other value
// (lists, scalars) is replaced by the delta's.
func Merge(base, delta map[string]any) map[string]any {
	out := map[string]any{}
	maps.Copy(out, base)
	for key, dv := range delta {
		if bv, ok := base[key].(map[string]any); ok {
			if dm, ok := dv.(map[string]any); ok {
				out[key] = Merge(bv, dm)
				continue
			}
		}
		out[key] = dv
	}
	return out
}
