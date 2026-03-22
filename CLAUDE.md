# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**gh-bench** is a Go CLI tool (GitHub CLI extension) for GitHub Actions CI benchmarking and failure extraction. It fetches workflow run logs, extracts numeric values for statistical analysis (`stats` command), and surfaces structured errors from failed runs using framework-aware parsers (`failures` command).

Module: `github.com/seanhalberthal/gh-bench` · Go 1.26.1

## Common Commands

```bash
make build                    # Build binary with version ldflags
make test                     # Tests with race detection + coverage
make test FILTER=TestName     # Run a single test by name
make check                    # vet + lint + test (full CI check)
make cover                    # HTML coverage report in browser
make fmt                      # gofmt all files
make install                  # Build + install as gh extension
```

## Architecture

### Command Layer (`cmd/`)
Cobra-based CLI with two subcommands:
- **stats** — extracts numeric values from CI logs via regex with named capture groups, computes aggregations (median, mean, p95, min, max)
- **failures** — fetches failed runs, auto-detects test framework, extracts structured failure details

Global `--json` flag on root command switches output format.

### Internal Packages

**`internal/runner/`** — GitHub Actions log fetching and parsing
- `GHExecutor` interface abstracts `gh` CLI calls (stubbed in tests via package-level `Executor` variable)
- `FetchLogs()` fetches concurrently using `errgroup` with configurable concurrency
- All log paths strip GitHub Actions `job\tstep\t` tab prefixes and ISO 8601 timestamps via `stripLogPrefixes()`
- `GetFailedSteps()` parses job/step JSON to isolate failed step logs
- `ExtractValues()` applies regex with named capture groups to extract numeric values; supports optional `(?P<label>...)` group for row-level context

**`internal/parser/`** — Framework-specific test failure parsers
- `FrameworkParser` interface: `Name()`, `Detect(logs)`, `Extract(logs)`
- Detection order: DotNet → Go → Vitest → Fallback (first match wins)
- Each parser uses regex to find failures, then looks backward/forward for error context
- Test data lives in `internal/parser/testdata/` (real CI log samples)

**`internal/config/`** — Project-level configuration
- Loads `.gh-bench.yml` from the working directory, walking up to the git root
- Provides defaults for `workflow` (all commands) and `failures.exclude-steps`
- CLI flags override config values when explicitly set

**`internal/stats/`** — Statistical aggregation
- `Compute(values, aggNames)` dispatches to median/mean/p95/min/max
- Immutable: creates copies before sorting to avoid mutating input

### Key Patterns
- **Executor injection**: `runner.Executor` is a package-level variable overridden in tests for deterministic, offline testing
- **Plugin architecture**: new framework parsers implement `FrameworkParser` and get added to the `parsers` slice in `parser.go`
- **Version injection**: `-X github.com/seanhalberthal/gh-bench/cmd.Version=$(VERSION)` via ldflags at build time
