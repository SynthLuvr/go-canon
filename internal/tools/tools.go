// Package tools pins the known-good external tool versions go-canon
// drives and resolves this build's own version.
//
// Every tool is a go.mod `tool` directive in the consumer's own module, so
// versions stay pinned by the consumer's go.mod/go.sum; these constants are
// what `go-canon doctor` compares against and `go-canon migrate` installs.
package tools

import (
	"regexp"
	"runtime/debug"
)

// Tool describes one external dev tool.
type Tool struct {
	// Name is the `go tool` invocation name.
	Name string
	// Pkg is the tool-directive package path for go.mod.
	Pkg string
	// Module is the module path owning Pkg, for `go list -m`.
	Module string
	// Version is the known-good version for this go-canon release.
	Version string
}

// GoTools are the tools go-canon itself drives.
var GoTools = []Tool{
	{
		Name:    "golangci-lint",
		Pkg:     "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		Module:  "github.com/golangci/golangci-lint/v2",
		Version: "v2.13.2",
	},
	{
		Name:    "govulncheck",
		Pkg:     "golang.org/x/vuln/cmd/govulncheck",
		Module:  "golang.org/x/vuln",
		Version: "v1.4.0",
	},
	{
		Name:    "modernize",
		Pkg:     "golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize",
		Module:  "golang.org/x/tools/gopls",
		Version: "v0.23.0",
	},
}

// TaskTool is the task runner consumers drive through Taskfile.yml. It is
// optional (go-canon commands work directly), so doctor only warns.
var TaskTool = Tool{
	Name:    "task",
	Pkg:     "github.com/go-task/task/v3/cmd/task",
	Module:  "github.com/go-task/task/v3",
	Version: "v3.53.1",
}

// CanonTool returns go-canon's own tool-directive entry pinned at
// version; migrate installs it into consumer repos.
func CanonTool(version string) Tool {
	return Tool{
		Name:    "go-canon",
		Pkg:     CanonPkg,
		Module:  "github.com/SynthLuvr/go-canon",
		Version: version,
	}
}

// CanonPkg is go-canon's own tool-directive package path.
const CanonPkg = "github.com/SynthLuvr/go-canon/cmd/go-canon"

// CanonVersion overrides the resolved build version when stamped at
// link time:
//
//	go build -ldflags "-X github.com/SynthLuvr/go-canon/internal/tools.CanonVersion=v0.4.0" ./cmd/go-canon
//
// The default marks an unstamped build.
var CanonVersion = "dev"

// releaseRe matches released versions: plain vX.Y.Z tags, not
// pre-releases or pseudo-versions.
var releaseRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// ResolveVersion returns this build's version: the ldflags stamp when
// present, else the module version carried in the build info of `go
// install ...@vX.Y.Z` and consumer `tool` builds, else "dev-<vcs
// revision>" (with -dirty for a modified tree) for local checkouts,
// else "dev".
func ResolveVersion() string {
	if CanonVersion != "dev" {
		return CanonVersion
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	v := "dev-" + rev[:min(7, len(rev))]
	if dirty {
		v += "-dirty"
	}
	return v
}

// CanonRelease returns the released version this binary was installed
// from, or "" for dev builds, pre-releases, and pseudo-versions —
// migrate only pins real releases.
func CanonRelease() string {
	if v := ResolveVersion(); releaseRe.MatchString(v) {
		return v
	}
	return ""
}

// GoFloor is the oldest go directive go-canon supports: the `tool`
// directive mechanism needs Go 1.24, and the pinned tool modules
// (golangci-lint, gopls) declare go 1.26 — `go mod tidy` enforces that
// maximum on go-canon and on every consumer that pins its toolchain,
// so migrate raises consumer modules to at least this.
const GoFloor = "1.26"

// GoVersion is the Go minor release this go-canon is validated against.
const GoVersion = "1.27"

// GoToolchain is the exact toolchain this go-canon is validated against
// (the `toolchain` directive and .go-version content, without the `go`
// prefix).
const GoToolchain = "1.27.1"

// PandocMin is the minimum pandoc version for the markdown gate.
const PandocMin = "3.10"
