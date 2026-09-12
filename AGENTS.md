# AGENTS.md

Instructions for AI coding agents working in this repository.

## Quick Start

``` bash
go run ./cmd/go-canon format # auto-format
go run ./cmd/go-canon lint   # static pipeline
go run ./cmd/go-canon test   # tests + coverage gate
```

or via Task: `go tool task check`.

## Required Workflow

Always run this before considering work complete:

``` bash
go build ./... && go tool task lint && go tool task test
```

All three must pass with zero errors.

## Toolchain

Lint, format, and environment checks come from go-canon itself — this
repo is self-hosted: the pipeline code is linted and tested by the
pipeline.

- Run tooling through `go tool task <task>` or
  `go run ./cmd/go-canon <command>`, not by invoking binaries from
  `go tool` directly.
- `go-canon lint` / `go-canon format` accept path arguments and `--fast`
  (skips govulncheck, dupl, and the markdown gate).
- Rule and preset changes belong in this repo (the embedded
  `internal/golangci/preset.yml` and the known-good versions in
  `internal/tools`); consumers only carry `go-canon.toml` deltas.
- Markdown files must be byte-identical to `pandoc --eol=lf -t gfm`
  output; fix drift with `go-canon format`.
- External tools are spawned only through resolved `go tool -n` paths
  (see `internal/execx`) — never through a shell or a bare PATH binary.

## Coding Conventions (Enforced)

These are not preferences — the pipeline fails if you violate them:

- gofumpt formatting; gci import order (std / external / local module)
- modern Go idioms: range-over-int, `slices`/`maps`/`min`/`max`, no
  `fmt.Sprintf` where `strconv` suffices (modernize + intrange +
  perfsprint)
- errors are values: checked (errcheck), wrapped at package boundaries
  (wrapcheck), compared with `errors.Is/As` (errorlint)
- every exported symbol and package documented (revive)
- table-driven tests; keep total statement coverage at or above 80%
- security findings from gosec must be fixed or justified with a
  targeted `// #nosec <rule>` comment carrying a reason

## Project Structure

- `cmd/go-canon/` — the command entry point (thin)
- `internal/app/` — command orchestration: lint, format, test, doctor,
  migrate pipelines
- `internal/config/` — `go-canon.toml` loading and delta semantics
- `internal/golangci/` — embedded preset + deep merge into the effective
  config
- `internal/execx/` — process execution (the `go tool -n` resolution
  seam)
- `internal/markdown/`, `internal/cover/`, `internal/module/`,
  `internal/tools/` — the individual gates and metadata
- `internal/testrunner/` — scriptable Runner fake for tests
- Tests are co-located (`*_test.go`), table-driven where it helps
