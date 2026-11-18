#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go test -run '^$' -bench='BenchmarkMatch/.+/(plain_maps|maps|plain_structs|structs_count)$' -benchmem .
