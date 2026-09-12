# go-canon

The shared Go toolchain — one meta-tool that owns every gate and preset,
in the same spirit as ts-canon (TypeScript) and canonist (Python).

Go modules make the bundling half of ts-canon unnecessary: since Go
1.24, `go.mod` `tool` directives give version-pinned, `go.sum`-verified
dev tooling compiled locally by the Go toolchain itself. go-canon is the
other half — orchestration, presets, gates, doctor, and migrate:

- `go-canon lint` — fail-fast pipeline: `go build`, golangci-lint
  (linters + formatters from a merged effective config), `modernize`,
  `go mod tidy -diff`, `govulncheck`, pandoc markdown
- `go-canon format` — `modernize -fix`, `golangci-lint fmt` (gofumpt +
  gci), `go mod tidy`, pandoc markdown rewrite
- `go-canon test` — `go test -race` with cross-package atomic coverage
  and an 80% statement gate
- `go-canon check` — lint, then test
- `go-canon doctor` — environment diagnostics (go, pinned tool versions,
  pandoc, OSV database reachability)
- `go-canon migrate` — adopt go-canon in an existing repo (`--dry-run`
  supported)

## Toolchain surface

A consumer’s `go.mod` pins everything; go-canon shells out through
`go tool` (resolved via `go tool -n`, spawned by absolute path — never
through a shell or a `.cmd`/`.ps1` shim):

``` go
tool (
    github.com/SynthLuvlr/go-canon/cmd/go-canon
    github.com/go-task/task/v3/cmd/task
    github.com/golangci/golangci-lint/v2/cmd/golangci-lint
    golang.org/x/vuln/cmd/govulncheck
    golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize
)
```

Task (go-task) is the task runner giving the one-command UX
(`task lint`, `task test`, `task check`, `task doctor`).

## Presets

golangci-lint v2 has no config inheritance, so go-canon ships the
canonical preset embedded and writes the *effective* config at
invocation time:

- the preset lives in `internal/golangci/preset.yml`
- local deltas go in `go-canon.toml` at the repo root, under
  `[golangci]` (deep-merge semantics: maps merge recursively, lists and
  scalars replace — same contract as canonist’s `[tool.canonist]`
  sections)
- at invocation, go-canon writes `.go-canon/golangci.yml` (gitignored),
  passes it via `--config`, and removes it afterwards (`--keep-config`
  to inspect)
- `[dupl] tokens` and `[test] coverage-threshold` are first-class sugar
  over the same merge

Rule and preset changes belong here, not in consumer repos: bump the
go-canon version and the whole toolchain moves together.

## Gates

| Gate           | go-canon                                   |
|----------------|--------------------------------------------|
| Coverage       | 80% statements (`-coverpkg=./...`, atomic) |
| Duplication    | dupl, 150-token threshold (tunable)        |
| SCA            | govulncheck (symbol-level reachability)    |
| SAST           | gosec via golangci-lint                    |
| Deps freshness | `go mod tidy -diff`                        |
| Format         | gofumpt + gci via golangci-lint            |
| Markdown       | pandoc `--eol=lf -t gfm`, byte-identical   |

`--fast` skips the slow gates (govulncheck, dupl, pandoc).

## Windows / managed endpoints

Every tool is compiled locally by the Go toolchain into the build cache
— there are no downloaded prebuilt binaries anywhere. go-canon spawns
children directly via `exec.Command` on resolved absolute paths, never
through a shell or a package-manager shim. Pandoc is the single system
dependency, verified by `doctor`.

## Self-hosting

This repo runs through itself (`go run ./cmd/go-canon check`); a module
cannot list itself in its own `tool` block, so the Taskfile builds the
local command instead of `go tool go-canon`.

## Development

``` bash
go run ./cmd/go-canon format   # apply gofumpt/gci/modernize/tidy/pandoc
go run ./cmd/go-canon lint     # the full pipeline
go run ./cmd/go-canon test     # tests + coverage gate
go run ./cmd/go-canon check    # lint + test
```
