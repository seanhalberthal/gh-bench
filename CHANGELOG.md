# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Optional `(?P<label>...)` capture group in patterns for row-level context (e.g. package names in `go-test` output)
- Common path prefix stripping in table output for readable labels

### Fixed
- `stats` command now strips GitHub Actions timestamps and `job\tstep\t` prefixes from full-run logs — presets like `go-test` and `duration` now match real CI output
- `jest` preset now matches Vitest `Duration` output in addition to Jest `Time:`
- `pytest` preset now handles `passed, N warnings in Xs` format

### Changed
- `go-test` preset captures the package name as a label, shown instead of the commit title
- Parser testdata updated to reflect cleaned (timestamp-stripped) log format

## [0.1.4]

### Added
- `--exclude-step` / `-x` flag on `failures` command to filter out CI orchestration steps by name
- `.gh-bench.yml` project config file for per-repo defaults (`workflow`, `failures.exclude-steps`)

## [0.1.3]

### Added
- Terminal spinner on stderr while fetching CI logs (auto-disabled in non-TTY environments)

### Changed
- Replaced briandowns/spinner with charmbracelet huh/spinner

## [0.1.1]

### Added
- Single-letter shorthand aliases for all CLI flags (e.g. `-w`, `-r`, `-l`, `-b`, `-f`)

### Changed
- `--list-presets` now has `-L` shorthand

## [0.1.0]

### Added
- `stats` command — extract numeric values from CI logs via regex with named capture groups, compute aggregations (median, mean, p95, min, max)
- `failures` command — fetch failed runs, auto-detect test framework, extract structured failure details
- Framework-aware parsers for .NET (xUnit/NUnit/MSTest), Go, and Vitest/Jest with fallback parser
- Concurrent log fetching via `gh api` with configurable concurrency
- `--json` global flag for machine-readable output
- Pattern presets for common CI metrics (`--preset` / `--list-presets`)
- Step filtering (`--step`) and branch filtering (`--branch`)
- Output formats: table, JSON, CSV
- GitHub Actions release workflow with auto-versioning
