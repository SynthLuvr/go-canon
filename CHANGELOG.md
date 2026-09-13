# Changelog

All notable changes to go-canon are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); releases are
annotated tags whose message carries the release notes.

## [Unreleased](https://github.com/SynthLuvr/gocanon/compare/v0.3.0...HEAD)

## [0.3.0](https://github.com/SynthLuvr/gocanon/compare/v0.2.0...v0.3.0) - 2026-09-13

### Changed

- Module path repaired to `github.com/SynthLuvr/gocanon` after the
  GitHub account was renamed from SynthLuvlr to SynthLuvr (the support
  appeal that unflagged the account also renamed it, which finally
  restores anonymous git, web, and proxy.golang.org access). v0.2.0’s
  `go.mod` still declared the pre-rename path, so the module could not
  resolve as `github.com/SynthLuvr/gocanon` — consumers must pin v0.3.0
  or later; import paths and `tool` directives update accordingly.

## [0.2.0](https://github.com/SynthLuvr/gocanon/compare/v0.1.2...v0.2.0) - 2026-09-13

### Changed

- Module and repository renamed to `github.com/SynthLuvr/gocanon`; the
  command is still `go-canon`. proxy.golang.org cached a fetch failure
  for the old module path from before the repository became public, so
  the rename gives the module a clean path in the public Go module
  ecosystem. Consumers must update import paths and `tool` directives.

## [0.1.2](https://github.com/SynthLuvr/gocanon/compare/v0.1.1...v0.1.2) - 2026-09-12

### Added

- MIT license (`LICENSE`) — go-canon was previously unlicensed, which
  blocked pkg.go.dev documentation and many adopters.
- `go-canon --version`, plus version reporting in the usage banner and
  `doctor`, resolved from build metadata: binaries built from a `tool`
  directive or `go install ...@vX.Y.Z` self-report their exact module
  version, local checkouts report `dev-<commit>` (with `-dirty` for a
  modified tree), and an ldflags stamp overrides both.
- `CHANGELOG.md` and a release workflow: pushing a `v*` tag runs the
  full pipeline and creates the GitHub Release from the annotated tag
  message — notes only, no prebuilt binaries.

### Changed

- The module `go` directive floor is now 1.26 (down from 1.27) — the
  minimum the pinned tool modules allow: golangci-lint v2.13.2 and gopls
  v0.23.0 declare `go 1.26.0`, and `go mod tidy` enforces that maximum
  on go-canon and every consumer pinning its toolchain. (The `tool`
  directive mechanism itself only needs 1.24, but that floor is
  unreachable without downgrading the validated tool versions.)
  Development and CI still build with the validated go1.27.1 toolchain.
- `migrate` raises consumer `go` directives to the 1.26 floor instead of
  1.27, and pins go-canon at the release the running binary was
  installed from; development builds warn and skip the self-pin.

### Fixed

- Self-reported version staleness: v0.1.1 binaries reported v0.1.0
  because the version was a hand-bumped constant; versions are now
  resolved from build metadata.

## [0.1.1](https://github.com/SynthLuvr/gocanon/compare/v0.1.0...v0.1.1) - 2026-09-12

### Added

- CI: Ubuntu + Windows matrix with pinned actions and go build caching,
  running on pull requests.

### Changed

- Tightened the app pipelines: lint, format, test, doctor, and migrate
  refinements.

### Fixed

- The Taskfile template written by `migrate`.

## [0.1.0](https://github.com/SynthLuvr/gocanon/releases/tag/v0.1.0) - 2026-09-12

Initial release: the shared Go toolchain — one meta-tool owning every
gate and preset. `lint`, `format`, `test`, `check`, `doctor`, and
`migrate` pipelines driven by consumer `go.mod` `tool` directives.
