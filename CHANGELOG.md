# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Changed
- `GetFailedSteps` decomposed into focused helpers (`collectFailedJobSteps`, `failedStepsFor`, `fetchJobLogsParallel`, `assembleStepResults`, `fetchJobsFromAPI`, `deduplicateByLatestAttempt`, `fetchJobsFromRunView`) to reduce cyclomatic complexity

### Fixed
- `failures` command now correctly surfaces jobs from re-run attempts — previously, jobs that failed in an earlier attempt and weren't re-run were silently dropped. The REST API is now queried with `filter=all` and deduplicated by job name keeping the latest attempt. Steps from re-runs are annotated with the attempt number (e.g. `Run tests (attempt 2)`)
- Vitest parser now detects `--typecheck` mode output (tsc-backed) and surfaces each TypeScript diagnostic as a failure — previously these logs fell through to the fallback parser

## [0.1.10]

### Added
- `grep` subcommand for searching CI run logs by keyword or regex

### Changed
- Failure timestamps now use the system's local timezone instead of hardcoded Europe/London

### Fixed
- Struct conversion used instead of literal for staticcheck S1016

## [0.1.9]

### Added
- Failure timestamps — `failures` output now includes the timestamp when each test failure occurred, extracted from raw CI log lines
- `--version` / `-v` flag on the root command to print the current version
- New `internal/logutil` package consolidating shared timestamp-stripping logic

### Fixed
- JSON and CSV output for `stats` and `failures` now uses `encoding/json` and `encoding/csv` instead of hand-rolled serialisation
- Test data race in `stubExecutor` resolved with mutex

## [0.1.8]

### Fixed
- `failures` command no longer treats cancelled steps as failures — only steps with a `failure` conclusion are reported
- Runs where every step was cancelled are now silently dropped instead of appearing as empty failures
- Spinner suppressed in non-TTY environments to avoid polluting captured output

## [0.1.7]

### Added
- `failures` command now filters to runs with open PRs by default, keeping output focused on actionable failures
- `--all` / `-a` flag on `failures` to include all failed runs (previous behaviour)
- Branch name shown in `failures` output (text, JSON, CSV)

### Changed
- `failures` display order reversed — most recent failures now appear at the bottom (closest to cursor)

## [0.1.6]

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
