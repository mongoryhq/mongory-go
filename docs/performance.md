# Performance & Benchmarks

## Reproducible Benchmarks

Run the benchmark script in `mongory-go`:

```bash
bash scripts/bench.sh
```

Or run `go test` directly:

```bash
go test -bench=BenchmarkMatch -benchmem .
```

### Measured Results (Apple M3, macOS arm64, `10,000` records per op)

| Case | `maps` (`ns/rec`) | `maps` allocs | `structs_count` (`ns/rec`) | `structs_count` allocs |
|---|---|---|---|---|
| `simple` (`age >= 18`) | `24.35 ns/rec` | `0 B/op`, `0 allocs/op` | `34.36 ns/rec` | `0 B/op`, `0 allocs/op` |
| `or` (`age >= 18` or `status == "active"`) | `38.95 ns/rec` | `0 B/op`, `0 allocs/op` | `58.11 ns/rec` | `0 B/op`, `0 allocs/op` |
| `nested_and` (`profile.city` + `status` + `age < 40`) | `43.05 ns/rec` | `0 B/op`, `0 allocs/op` | `62.06 ns/rec` | `0 B/op`, `0 allocs/op` |
| `in_array_field` (`tags $in ["go", "rust"]`) | `44.29 ns/rec` | `0 B/op`, `0 allocs/op` | `75.52 ns/rec` | `0 B/op`, `0 allocs/op` |
| `regex` (`name $regex "^user[a-f]$"`) | `44.15 ns/rec` | `0 B/op`, `0 allocs/op` | `58.00 ns/rec` | `0 B/op`, `0 allocs/op` |

> For cross-language benchmarks (`mongory-core`, `mongory-js`, `mongory-rb`), see [Mongory Central](https://mongoryhq.github.io/mongory-central/#benchmarks).
