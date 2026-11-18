# Performance & Benchmarks

## Reproducible Benchmarks

Run the benchmark script in `mongory-go`:

```bash
bash scripts/bench.sh
```

Or run `go test` directly:

```bash
go test -run '^$' -bench=BenchmarkMatch -benchmem .
```

### Measured Results (Apple M3, macOS arm64, `10,000` records per op, `0 B/op` / `0 allocs/op`)

| Case | Plain Go (`maps`) | `mongory-go` (`maps`) | Map Overhead | Plain Go (`structs`) | `mongory-go` (`structs_count`) |
|---|---|---|---|---|---|
| `simple` | `11.54 ns/rec` | `25.27 ns/rec` | `2.19x` | `4.17 ns/rec` | `35.44 ns/rec` |
| `or` | `20.94 ns/rec` | `39.61 ns/rec` | `1.89x` | `2.19 ns/rec` | `58.19 ns/rec` |
| `nested_and` | `21.87 ns/rec` | `42.28 ns/rec` | `1.93x` | `2.63 ns/rec` | `62.30 ns/rec` |
| `in_array_field` | `29.24 ns/rec` | `46.09 ns/rec` | `1.58x` | `4.37 ns/rec` | `76.12 ns/rec` |
| `large_in` (500 items) | `15.75 ns/rec` | `27.35 ns/rec` | `1.74x` | `11.37 ns/rec` | `44.13 ns/rec` |
| `regex` | `35.97 ns/rec` | `44.56 ns/rec` | `1.24x` | `29.96 ns/rec` | `57.49 ns/rec` |

> For cross-language benchmarks (`mongory-core`, `mongory-js`, `mongory-rb`), see [Mongory Central](https://mongoryhq.github.io/mongory-central/#benchmarks).
