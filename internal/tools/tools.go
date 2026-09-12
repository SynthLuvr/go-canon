// Package tools pins the known-good external tool versions go-canon drives.
//
// Every tool is a go.mod `tool` directive in the consumer's own module, so
// versions stay pinned by the consumer's go.mod/go.sum; these constants are
// what `go-canon doctor` compares against and `go-canon migrate` installs.
package tools

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

// CanonTool is go-canon itself as a tool-directive entry, installed by
// migrate into consumer repos.
var CanonTool = Tool{
	Name:    "go-canon",
	Pkg:     CanonPkg,
	Module:  "github.com/SynthLuvlr/go-canon",
	Version: CanonVersion,
}

// CanonPkg is go-canon's own tool-directive package path.
const CanonPkg = "github.com/SynthLuvlr/go-canon/cmd/go-canon"

// CanonVersion is this go-canon release.
const CanonVersion = "v0.1.0"

// GoVersion is the Go minor release this go-canon is validated against.
const GoVersion = "1.27"

// GoToolchain is the exact toolchain this go-canon is validated against
// (the `toolchain` directive and .go-version content, without the `go`
// prefix).
const GoToolchain = "1.27.1"

// PandocMin is the minimum pandoc version for the markdown gate.
const PandocMin = "3.10"
