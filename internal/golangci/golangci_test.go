package golangci

import (
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestMergeMapsRecursively(t *testing.T) {
	base := map[string]any{
		"linters": map[string]any{
			"enable": []any{"errcheck", "govet"},
			"settings": map[string]any{
				"dupl": map[string]any{"threshold": 150},
			},
		},
	}
	delta := map[string]any{
		"linters": map[string]any{
			"settings": map[string]any{
				"dupl": map[string]any{"threshold": 200},
			},
		},
	}
	got := Merge(base, delta)
	linters := got["linters"].(map[string]any)
	settings := linters["settings"].(map[string]any)
	dupl := settings["dupl"].(map[string]any)
	if dupl["threshold"] != 200 {
		t.Errorf("merged dupl threshold = %v, want 200", dupl["threshold"])
	}
	enable, ok := linters["enable"].([]any)
	if !ok || len(enable) != 2 {
		t.Errorf("merged enable = %v, want the untouched base list", linters["enable"])
	}
}

func TestMergeListsReplace(t *testing.T) {
	base := map[string]any{"enable": []any{"a", "b"}}
	delta := map[string]any{"enable": []any{"c"}}
	got := Merge(base, delta)
	enable, ok := got["enable"].([]any)
	if !ok || len(enable) != 1 || enable[0] != "c" {
		t.Errorf("Merge() enable = %v, want [c] (lists replace, not append)", got["enable"])
	}
}

func TestMergeScalarsReplaceAndAdd(t *testing.T) {
	base := map[string]any{"a": 1}
	delta := map[string]any{"a": 2, "b": true}
	got := Merge(base, delta)
	if got["a"] != 2 {
		t.Errorf("Merge() a = %v, want 2", got["a"])
	}
	if got["b"] != true {
		t.Errorf("Merge() b = %v, want true", got["b"])
	}
}

func TestMergeDoesNotMutateInputs(t *testing.T) {
	base := map[string]any{"x": map[string]any{"k": 1}}
	delta := map[string]any{"x": map[string]any{"k": 2}}
	_ = Merge(base, delta)
	if base["x"].(map[string]any)["k"] != 1 {
		t.Error("Merge() mutated base")
	}
	if delta["x"].(map[string]any)["k"] != 2 {
		t.Error("Merge() mutated delta")
	}
}

func TestMergeReplacesMapWithScalar(t *testing.T) {
	base := map[string]any{"x": map[string]any{"k": 1}}
	delta := map[string]any{"x": "flat"}
	if got := Merge(base, delta); got["x"] != "flat" {
		t.Errorf("Merge() x = %v, want flat", got["x"])
	}
}

func TestEffectiveEmptyDeltaMatchesPreset(t *testing.T) {
	out, err := Effective(nil)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("effective config does not parse: %v", err)
	}
	if err := yaml.Unmarshal(preset, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Effective(nil) differs from preset:\ngot:  %v\nwant: %v", got, want)
	}
}

func TestEffectiveAppliesDelta(t *testing.T) {
	delta := map[string]any{
		"linters": map[string]any{
			"settings": map[string]any{
				"dupl": map[string]any{"threshold": 250},
			},
		},
	}
	out, err := Effective(delta)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	linters := got["linters"].(map[string]any)
	settings := linters["settings"].(map[string]any)
	dupl := settings["dupl"].(map[string]any)
	if dupl["threshold"] != 250 {
		t.Errorf("effective dupl threshold = %v, want 250", dupl["threshold"])
	}
}

func TestEffectiveParsesWithGolangciSemantics(t *testing.T) {
	// The preset itself must be a well-formed golangci-lint v2 config:
	// a version key plus linters/formatters sections.
	var got map[string]any
	out, err := Effective(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got["version"] != "2" {
		t.Errorf("effective version = %v, want 2", got["version"])
	}
	for _, section := range []string{"linters", "formatters"} {
		if _, ok := got[section].(map[string]any); !ok {
			t.Errorf("effective config missing %s section: %v", section, got)
		}
	}
}
