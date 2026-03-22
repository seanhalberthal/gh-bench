<div align="center">

<br>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset=".github/assets/logo-light.svg">
  <img alt="bench" src=".github/assets/logo-dark.svg" width="320">
</picture>

<br>

**Extract CI metrics and surface test failures from GitHub Actions logs.**

[![CI](https://img.shields.io/github/actions/workflow/status/seanhalberthal/gh-bench/release.yml?style=flat&logo=githubactions&logoColor=white&label=CI)](https://github.com/seanhalberthal/gh-bench/actions)
[![Release](https://img.shields.io/github/v/release/seanhalberthal/gh-bench?style=flat&logo=github&logoColor=white)](https://github.com/seanhalberthal/gh-bench/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![gh extension](https://img.shields.io/badge/gh-extension-2088FF?style=flat&logo=githubactions&logoColor=white)](https://cli.github.com/manual/gh_extension)
[![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black)](https://github.com/seanhalberthal/gh-bench/releases)
[![macOS](https://img.shields.io/badge/macOS-000000?style=flat&logo=apple&logoColor=white)](https://github.com/seanhalberthal/gh-bench/releases)
[![Windows](https://img.shields.io/badge/Windows-0078D4?style=flat&logo=windows&logoColor=white)](https://github.com/seanhalberthal/gh-bench/releases)

[Quick Start](#quick-start) · [Commands](#commands) · [Framework Support](#framework-support) · [Examples](#examples)

</div>

---

A [`gh`](https://cli.github.com) CLI extension that pulls numeric values from CI run logs for benchmarking and aggregates stats (median, mean, p95), and extracts structured test failures with framework-aware parsers.

## Quick Start

```bash
# Install
gh extension install seanhalberthal/gh-bench

# Aggregate a metric across recent runs
gh bench stats --workflow ci.yml --pattern 'duration=(?P<ms>[0-9.]+)ms'

# Surface test failures from a specific run
gh bench failures --runs 12345678
```

---

## Commands

### `gh bench stats`

Extract a numeric value from logs across multiple workflow runs and compute aggregations.

```bash
gh bench stats --workflow ci.yml --pattern 'median=(?P<ms>[0-9.]+)ms'
gh bench stats --workflow ci.yml --pattern 'duration=(?P<s>[0-9.]+)s' --agg median,p95,min,max
gh bench stats --runs 111,222,333 --pattern 'score=(?P<val>[0-9.]+)'
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--workflow` | `-w` | | Workflow filename or name |
| `--runs` | `-r` | | Comma-separated list of run IDs |
| `--pattern` | `-P` | | Regex with a named capture group `(?P<name>...)` |
| `--preset` | `-p` | | Use a built-in pattern preset (see `--list-presets`) |
| `--list-presets` | `-L` | | List available pattern presets and exit |
| `--limit` | `-l` | `10` | Max number of runs to fetch |
| `--branch` | `-b` | | Filter runs by branch |
| `--agg` | `-a` | `median` | Aggregations: `median`, `mean`, `p95`, `min`, `max` (comma-separated) |
| `--match` | `-m` | `first` | Which matches to extract per run: `first`, `all` |
| `--step` | `-s` | | Filter logs to a specific step name (substring match) |
| `--concurrency` | `-c` | `5` | Concurrent log fetchers |
| `--format` | `-f` | `table` | Output format: `table`, `json`, `csv` |

Either `--pattern` or `--preset` is required. Either `--workflow` or `--runs` is required.

### `gh bench failures`

Fetch failed CI runs, identify failing steps, and extract structured errors using framework-aware parsers.

```bash
gh bench failures --workflow ci.yml
gh bench failures -r 12345678
gh bench failures -w ci.yml -b main -l 10
gh bench failures --runs 12345678 --format json
gh bench failures -w ci.yml --group
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--workflow` | `-w` | | Workflow filename or name |
| `--runs` | `-r` | | Comma-separated list of run IDs |
| `--limit` | `-l` | `5` | Max failed runs to fetch |
| `--branch` | `-b` | | Filter by branch |
| `--group` | `-g` | `false` | Group identical failures across runs |
| `--concurrency` | `-c` | `5` | Concurrent log fetchers |
| `--format` | `-f` | `table` | Output format: `table`, `json`, `csv` |

Either `--workflow` or `--runs` is required.

---

## Framework Support

The `failures` command auto-detects the test framework from log output and extracts structured failure information.

| Framework | Detection | Extracted Fields |
|-----------|-----------|------------------|
| **.NET** (xUnit, NUnit, MSTest) | `Failed TestName [duration]`, `error CS*:`, summary lines | Test name, duration, exception, assertion, stack trace location |
| **Go** | `--- FAIL: TestName (duration)`, `FAIL\tpackage` | Test name, duration, error message, file:line |
| **Vitest** / Jest | `✗ Suite > Test`, `FAIL *.test.tsx` | Test name, error type, expected/actual, file:line:col |
| **Fallback** | No framework detected | Last 30 non-boilerplate lines, `##[error]` messages |

The fallback parser strips GitHub Actions boilerplate (env vars, annotations, cleanup lines, shell script source) and prioritises `##[error]` messages.

---

## Examples

### Tracking build duration across runs

```bash
$ gh bench stats --workflow ci.yml --pattern 'Total time:\s*(?P<seconds>[0-9.]+)s' --agg median,p95

RUN ID          TITLE                           VALUE
23364348137     DANA-1338 Replace law firm ...   84.5
23348219428     DANA-1335 Add Roslyn analys...   91.0
23341291675     DANA-1332 Rename hooks ...       103.2
────────────────────────────────────────────────────────────────────
median: 91.0  p95: 103.2
```

### Extracting .NET test failures

```bash
$ gh bench failures --runs 23341983210

● RUN 23341983210 — DANA-1335 Add Roslyn analyser (2026-03-20T12:03:31Z)
  Step: Run integration-platform tests
  Framework: dotnet

  Failed Tests (1)

  ✗ Dana.IntegrationTests.TokenManagement.ExchangeCodeForTokens_ReturnsBadRequest [1 s]
      Shouldly.ShouldAssertException : errors[0].Reason
      should start with "Microsoft 365 authentication fai"
      but was "An unexpected error occurred..."
      at ExchangeCodeForTokensIntegrationTests.cs:line 181
```

### JSON output for scripting

```bash
$ gh bench failures --runs 23341983210 --json | jq '.[].failures[].test_name'
"Dana.IntegrationTests.TokenManagement.ExchangeCodeForTokens_ReturnsBadRequest"
```

---

## Building from Source

```bash
git clone https://github.com/seanhalberthal/gh-bench
cd gh-bench
make build      # Build the binary
make install    # Build and install as gh extension
make test       # Run tests with race detection + coverage
make check      # Run all checks (vet, lint, test)
```

<details>
<summary>All Makefile targets</summary>

| Target | Description |
|--------|-------------|
| `build` | Build the binary |
| `install` | Build and install as gh extension |
| `uninstall` | Remove the gh extension |
| `clean` | Remove build artefacts |
| `test` | Run tests with race detection and coverage |
| `cover` | Show coverage report in browser |
| `lint` | Run golangci-lint |
| `fmt` | Format all Go files |
| `vet` | Run go vet |
| `check` | Run all checks (vet, lint, test) |

Use `FILTER="TestName"` to run specific tests: `make test FILTER=TestDotnet`

</details>
