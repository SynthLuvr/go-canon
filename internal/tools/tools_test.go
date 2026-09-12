package tools

import (
	"strings"
	"testing"
)

// withCanonVersion stamps a fake version for the test's duration.
func withCanonVersion(t *testing.T, version string) {
	t.Helper()
	orig := CanonVersion
	CanonVersion = version
	t.Cleanup(func() { CanonVersion = orig })
}

func TestResolveVersionStampWins(t *testing.T) {
	withCanonVersion(t, "v9.9.9")
	if got := ResolveVersion(); got != "v9.9.9" {
		t.Errorf("ResolveVersion() = %q, want the stamped v9.9.9", got)
	}
}

func TestResolveVersionDevFallback(t *testing.T) {
	// Test binaries have no module version; a vcs stamp may or may not
	// be present depending on where the test runs, so anything
	// dev-prefixed is correct.
	withCanonVersion(t, "dev")
	got := ResolveVersion()
	if !strings.HasPrefix(got, "dev") {
		t.Errorf("ResolveVersion() = %q, want a dev-prefixed fallback", got)
	}
}

func TestCanonRelease(t *testing.T) {
	tests := []struct {
		name  string
		stamp string
		want  string
	}{
		{"release", "v0.1.2", "v0.1.2"},
		{"dev", "dev", ""},
		{"pseudo-version", "v0.0.0-20260912000000-abcdefabcdef", ""},
		{"pre-release", "v0.2.0-rc.1", ""},
		{"not a version", "custom-build", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withCanonVersion(t, tt.stamp)
			if got := CanonRelease(); got != tt.want {
				t.Errorf("CanonRelease() = %q, want %q", got, tt.want)
			}
		})
	}
}
