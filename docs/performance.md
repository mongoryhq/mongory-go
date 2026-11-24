# Performance & Benchmarks

## Reproducible Benchmarks

Run the zero-dependency benchmark script in `mongory-go`:

```bash
bash scripts/bench.sh
```

Or run `go test` directly:

```bash
go test -run '^$' -bench=BenchmarkMatch -benchmem .
```

### Plain Go vs `mongory-go` (Apple M3, macOS arm64, `10,000` records per op, `0 B/op` / `0 allocs/op`)

| Case | Plain Go (`maps`) | `mongory-go` (`maps`) | Map Overhead | Plain Go (`structs`) | `mongory-go` (`structs_count`) |
|---|---:|---:|---:|---:|---:|
| `simple` | `11.54 ns/rec` | `25.27 ns/rec` | `2.19x` | `4.17 ns/rec` | `35.44 ns/rec` |
| `or` | `20.94 ns/rec` | `39.61 ns/rec` | `1.89x` | `2.19 ns/rec` | `58.19 ns/rec` |
| `nested_and` | `21.87 ns/rec` | `42.28 ns/rec` | `1.93x` | `2.63 ns/rec` | `62.30 ns/rec` |
| `in_array_field` | `29.24 ns/rec` | `46.09 ns/rec` | `1.58x` | `4.37 ns/rec` | `76.12 ns/rec` |
| `large_in` (500 items) | `15.75 ns/rec` | `27.35 ns/rec` | `1.74x` | `11.37 ns/rec` | `44.13 ns/rec` |
| `regex` | `35.97 ns/rec` | `44.56 ns/rec` | `1.24x` | `29.96 ns/rec` | `57.49 ns/rec` |

### Peer Go Library Comparison (`bench/` Submodule)

To keep the root `mongory-go` module strictly zero-dependency, peer competitor benchmarks live in the isolated [`bench/`](../bench) Go module:

```bash
cd bench && go test -run '^$' -bench=BenchmarkCompetitors -benchmem -benchtime=500ms .
```

Measured against `github.com/256dpi/lungo/mongokit` (`v0.4.0`) and `github.com/SierraSoftworks/connor` (`v1.0.2`) on `10,000` records per op (Apple M3, macOS arm64):

| Case | `mongory-go` (`0 B/op`, `0 allocs/op`) | `lungo/mongokit` (`10k–16k allocs/op`) | `connor` (`10k–21k allocs/op`) | `mongory` vs `lungo` | `mongory` vs `connor` |
|---|---:|---:|---:|---:|---:|
| `simple` | `30.16 ns/rec` | `55.41 ns/rec` | `121.7 ns/rec` | **1.84x faster** | **4.04x faster** |
| `or` | `44.92 ns/rec` | `109.7 ns/rec` | `239.9 ns/rec` | **2.44x faster** | **5.34x faster** |
| `nested_and` | `47.71 ns/rec` | `74.05 ns/rec` | `169.4 ns/rec` | **1.55x faster** | **3.55x faster** |
| `in_array_field` | `49.14 ns/rec` | `85.10 ns/rec` | *(not supported)* | **1.73x faster** | — |
| `large_in_500` | `28.14 ns/rec` | `2,474 ns/rec` | `4,855 ns/rec` | **87.9x faster** | **172.5x faster** |
| `regex` | `45.96 ns/rec` | `52.27 ns/rec` | *(not supported)* | **1.14x faster** | — |

*(Note: `lungo/mongokit` requires BSON `primitive.Regex` for regex queries; `connor` uses `$ge` instead of `$gte`, nested maps instead of dot-paths, and does not support array `$in` intersection or `$regex`.)*

> For cross-language benchmarks (`mongory-core`, `mongory-js`, `mongory-rb`, `mongory-py`), see [Mongory Central](https://mongoryhq.github.io/mongory-central/#benchmarks).
