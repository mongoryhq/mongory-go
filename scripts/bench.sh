#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

go test -bench='BenchmarkMatch/(simple|or|nested_and|in_array_field|large_in_500|regex)/(maps|structs|structs_count)' -benchmem .
