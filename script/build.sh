#!/usr/bin/env bash
set -e

# Build script for gh-bench extension.
# Used by the gh extension precompile action.

VERSION="${GH_BENCH_VERSION:-dev}"
LDFLAGS="-s -w -X github.com/seanhalberthal/gh-bench/cmd.Version=${VERSION}"

go build -trimpath -ldflags "${LDFLAGS}" -o gh-bench .
