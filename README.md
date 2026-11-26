# mongory-go

[![Go Reference](https://pkg.go.dev/badge/github.com/mongoryhq/mongory-go.svg)](https://pkg.go.dev/github.com/mongoryhq/mongory-go)
[![Release](https://img.shields.io/github/v/release/mongoryhq/mongory-go)](https://github.com/mongoryhq/mongory-go/releases)
[![CI](https://github.com/mongoryhq/mongory-go/actions/workflows/ci.yml/badge.svg)](https://github.com/mongoryhq/mongory-go/actions/workflows/ci.yml)
[![mongory-spec](https://github.com/mongoryhq/mongory-go/actions/workflows/spec-conformance.yml/badge.svg)](https://github.com/mongoryhq/mongory-go/actions/workflows/spec-conformance.yml)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Zero-dependency, pure Go in-memory MongoDB-style query and rule engine** with `0 allocs/op` map/struct hot paths, built-in `Explain` / `Trace` debugging, and cross-language parity via [`mongory-spec`](https://github.com/mongoryhq/mongory-spec).

**[Try the Interactive Playground & Visual Explain/Trace Debugger ↗](https://mongoryhq.github.io/mongory-central/)**

[![Mongory Playground & Visual Trace](https://mongoryhq.github.io/mongory-central/playground-trace.png)](https://mongoryhq.github.io/mongory-central/)

> **Ecosystem:** [Playground](https://mongoryhq.github.io/mongory-central/) · [`mongory-js` (TypeScript/JS)](https://github.com/mongoryhq/mongory-js) · [`mongory-py` (Python)](https://github.com/mongoryhq/mongory-py) · [`mongory-rb` (Ruby)](https://github.com/mongoryhq/mongory-rb) · [`mongory-core` (C99)](https://github.com/mongoryhq/mongory-core) · [`mongory-spec`](https://github.com/mongoryhq/mongory-spec)

## Installation

```bash
go get github.com/mongoryhq/mongory-go
```

Requires Go `1.22+` (pure Go, zero external dependencies, no CGO).

## Quick Start

Compile a filter once with `Compile` (or `MustCompile` for static conditions), then evaluate maps, BSON documents, slices, or Go structs directly with `Match`, `Filter`, `Find`, `Any`, or `Count`:

```go
package main

import (
	"fmt"

	"github.com/mongoryhq/mongory-go"
)

type User struct {
	Name   string   `bson:"name"`
	Age    int      `json:"age"`
	Tags   []string `bson:"tags"`
	Active bool     // no tag: key is "Active" (exact Go field name)
}

var adults = mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})

func main() {
	users := []User{
		{Name: "Ann", Age: 30, Tags: []string{"go"}, Active: true},
		{Name: "Bob", Age: 15, Tags: []string{"ruby"}, Active: true},
		{Name: "Cid", Age: 42, Tags: []string{"go", "c"}},
	}

	m, err := mongory.Compile(mongory.M{
		"Active": true,
		"$or": mongory.A{
			mongory.M{"tags": "go"},
			mongory.M{"age": mongory.M{"$lt": 18}},
		},
	})
	if err != nil {
		panic(err) // errors.Is(err, mongory.ErrInvalidCondition)
	}

	matched := mongory.Filter(users, m) // []User{Ann, Bob}
	first, ok := mongory.Find(users, adults)
	fmt.Println(len(matched), first.Name, ok, adults.Match(users[1]))
}
```

One-shot matching is also available via `ok, err := mongory.Match(cond, record)`, and compile-time checked operator constants are available via `github.com/mongoryhq/mongory-go/op` (`op.Gte`, `op.In`, `op.Or`, `op.Regex`, etc.).

## Why Mongory? (and When Not to Use It)

- **Use Mongory when**: rules are loaded dynamically from JSON/DB/config (feature flags, webhook routing, user-defined filters), shared across Go + TS/Python/Ruby services with identical semantics, or need `Explain()` / `Trace()` debugging without pulling in a database or heavy VM.
- **Don't use Mongory when**: your filter is a static Go `if` statement hardcoded in source code, or when you need a full policy/expression language with arithmetic and custom functions (consider CEL or OPA Rego) or MongoDB aggregation pipelines (`$group`, `$lookup`).
- **Pure Go by design**: no CGO overhead, no cross-compile friction, direct struct field/tag reading (`mongory > bson > json`), linear-time Go RE2 `$regex`, and `0 allocs/op` on map and struct `Count` hot paths.

## Benchmark Snapshot

Measured on Apple M3 (macOS arm64, `10,000` records per op; full Plain Go & peer competitor tables in [docs/performance.md](docs/performance.md)):

| Case | `mongory-go` (`0 B/op`, `0 allocs/op`) | `lungo/mongokit` (`v0.4.0`) | `connor` (`v1.0.2`) | `mongory` vs `lungo` | `mongory` vs `connor` |
|---|---:|---:|---:|---:|---:|
| `simple` | `30.16 ns/rec` | `55.41 ns/rec` | `121.7 ns/rec` | **1.84x faster** | **4.04x faster** |
| `or` | `44.92 ns/rec` | `109.7 ns/rec` | `239.9 ns/rec` | **2.44x faster** | **5.34x faster** |
| `nested_and` | `47.71 ns/rec` | `74.05 ns/rec` | `169.4 ns/rec` | **1.55x faster** | **3.55x faster** |
| `large_in_500` | `28.14 ns/rec` | `2,474 ns/rec` | `4,855 ns/rec` | **87.9x faster** | **172.5x faster** |

*(Map hot paths run at `25.27 ns/rec` simple and `27.35 ns/rec` for 500-item `$in`; struct `Count` runs at `35.44 ns/rec` simple, all at `0 allocs/op`. Peer benchmarks live in the isolated [`bench/`](bench) submodule so the root module stays zero-dependency.)*

## Debugging with `Explain`, `Trace`, `ExplainTree`, and `TraceTree`

`Explain()`, `Trace(record)`, `ExplainTree()`, and `TraceTree(record)` are pure, non-mutating, and safe to call concurrently with `Match`. The `*Tree` variants return JSON-serializable `*ExplainNode` / `*TraceNode` structs (`Kind`, `Status`, `Label`, `Priority`, `Operator`, `Field`, `Index`, `SkippedCount`, `ValueText`, `Value`, `Exists`, `Children`):

```go
fmt.Print(m.Explain())

ok, trace := m.Trace(mongory.M{"age": 15, "status": "active"})
fmt.Print(trace)

ok, tree := m.TraceTree(mongory.M{"age": 15, "status": "active"})
data, _ := json.MarshalIndent(tree, "", "  ")
```

```text
[FAIL] And | value={"age": 15, "status": "active"}
├── [PASS] Field("status") | value="active"
│   └── [PASS] Eq("active") | value="active"
└── [FAIL] Field("age") | value=15
    └── [FAIL] Gte(18) | value=15
```

## Composing & Extending per Compile

Use `WithOptions` for reusable option presets (no global mutable state), `And`/`Or` + `FilterLimit` for dynamic queries, and `ErrInvalidOption` vs `ErrInvalidCondition` to separate setup errors from untrusted user filters:

```go
var DomainOptions = mongory.WithOptions(
	mongory.WithConverter(func(id primitive.ObjectID) any { return id.Hex() }),
	mongory.WithField("minor_units", func(c *Condition) any { return c.GetMinPrice().GetUnits() }),
	mongory.WithOperator("$moneyGte", mongory.Operator{
		MatchElements: true,
		Compile: func(arg any) (mongory.Predicate, error) {
			target, ok := arg.(*money.Money)
			if !ok || target == nil {
				return nil, errors.New("$moneyGte requires *money.Money")
			}
			return func(v any, exists bool) bool {
				rec, ok := v.(*money.Money)
				return exists && ok && rec != nil && rec.GetCurrencyCode() == target.GetCurrencyCode() && compareMoney(rec, target) >= 0
			}, nil
		},
	}),
)

cond := mongory.And(
	mongory.M{"merchant_id": merchantID},
	mongory.M{"min_price": mongory.M{"$gte": 100, "$moneyGte": targetMoney}},
)
m := mongory.MustCompile(cond, DomainOptions)
top10 := mongory.FilterLimit(records, m, 10)
```

<details>
<summary>Operator constants example (<code>mongory-go/op</code>)</summary>

```go
import (
	"github.com/mongoryhq/mongory-go"
	"github.com/mongoryhq/mongory-go/op"
)

m := mongory.MustCompile(mongory.M{
	"age": mongory.M{op.Gte: 18, op.Lt: 65},
	op.Or: mongory.A{
		mongory.M{"role": mongory.M{op.In: mongory.A{"admin", "editor"}}},
		mongory.M{"name": mongory.M{op.Regex: "^A", op.Options: "i"}},
	},
})
```

</details>

## Semantics, Differences & Conformance

- **Conditions & Records**: Accepts `mongory.M`, `map[string]any`, `bson.M`/`bson.D`, structs or struct pointers, slices, or scalars. Dot paths (`"address.city"`), escaped dots (`"a\\.b"`), array indexes (`"tags.0"`, `"tags.-1"`), and MongoDB implicit array matching apply. `ObjectID` (`[N]byte`) and `DateTime` (`Time() time.Time`) work without importing a BSON driver.
- **Struct field keys**: Resolve tags in priority order (`mongory` > `bson` > `json` > bare tag), falling back to the exact Go field name (no lowercasing). `,inline` struct/map fields are flattened.
- **Operators (17)**: `$eq $ne $gt $gte $lt $lte $in $nin $exists $present $and $or $not $elemMatch $every $size $regex` (`$options`: `i`, `m`, `s`). Unknown `$` operators fail fast at `Compile` time.
- **Differences from MongoDB**: `$every` (returns `false` on `[]`) and `$present` are Mongory extensions; `$regex` uses Go RE2 within the cross-engine `i`/`m`/`s` subset. See [`mongory-spec` Differences from MongoDB](https://github.com/mongoryhq/mongory-spec#37-differences-from-mongodb-query-syntax).

```bash
MONGORY_SPEC_DIR=../mongory-spec go test -v ./...
bash scripts/bench.sh
```

## Security & Untrusted Input

- **Engine protections**: Pure in-memory evaluation (no SQL, no `$where`/`eval`), linear-time Go RE2 `$regex` (immune to ReDoS), cycle detection, and configurable `WithMaxDepth` (default `64`) / `WithMaxNodes` (default `4096`) compile limits.
- **Application responsibility**: If you accept query conditions from untrusted callers, validate that expected scalar values are not operator maps (to prevent NoSQL-style operator injection such as `{"$ne": null}`) and allowlist queryable fields before evaluating records containing sensitive data.

## Versioning & License

`v0.x` while the API is evolving; `v1` once stable; `v2+` via semantic import paths. Released under the [MIT License](LICENSE).
