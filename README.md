# mongory-go

**Zero-dependency Go in-memory Mongo-style rule engine & query matcher** with zero-allocation map/struct evaluation and built-in **Explain & Trace** debugging — part of the [`mongory-spec`](https://github.com/mongoryhq/mongory-spec) cross-language ecosystem.

**[Try the Interactive Playground & Visual Explain/Trace Debugger ↗](https://mongoryhq.github.io/mongory-central/)**

[![Mongory Playground & Visual Trace](https://mongoryhq.github.io/mongory-central/playground-trace.png)](https://mongoryhq.github.io/mongory-central/)

> **Zero database required:** Compile a filter condition once into an immutable, concurrent-safe `*mongory.Matcher` and evaluate maps, BSON documents, slices, or Go structs in memory.  
> **Ecosystem:** [Interactive Playground](https://mongoryhq.github.io/mongory-central/) · [`mongory-js` (TypeScript/JS)](https://github.com/mongoryhq/mongory-js) · [`mongory-rb` (Ruby)](https://github.com/mongoryhq/mongory-rb) · [`mongory-core` (C99)](https://github.com/mongoryhq/mongory-core) · [`mongory-spec`](https://github.com/mongoryhq/mongory-spec)

## Installation

```bash
go get github.com/mongoryhq/mongory-go
```

Requires Go 1.22 or later (pure Go, zero external dependencies).

## Usage

Compile a filter once with `Compile` (or `MustCompile` for static conditions in source code), then evaluate records with `Match`, `Filter`, `Find`, `Any`, or `Count`.

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

### One-shot Matching

```go
ok, err := mongory.Match(mongory.M{"age": mongory.M{"$gte": 18}}, record)
```

### Composing & Extending per Compile

Use `WithOptions` to define reusable option presets (no global mutable state), `And`/`Or` + `FilterLimit` for dynamic queries, and `ErrInvalidOption` vs `ErrInvalidCondition` to separate setup errors from untrusted user filters:

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

// Validate preset once at init: var _ = mongory.MustCompile(nil, DomainOptions)

cond := mongory.And(
	mongory.M{"merchant_id": merchantID},
	mongory.M{"min_price": mongory.M{"$gte": 100, "$moneyGte": targetMoney}},
)
m := mongory.MustCompile(cond, DomainOptions)
top10 := mongory.FilterLimit(records, m, 10)
```

### Debugging with `Explain` and `Trace`

Both `Explain()` and `Trace(record)` are pure, non-mutating, and safe to call concurrently with `Match` (see the interactive visual AST & short-circuit tree in the [Live Playground](https://mongoryhq.github.io/mongory-central/#playground)):

```go
fmt.Print(m.Explain())

ok, trace := m.Trace(mongory.M{"age": 15, "status": "active"})
fmt.Print(trace)
```

Example `Trace` output:

```text
[FAIL] And | value={"age": 15, "status": "active"}
├── [PASS] Field("status") | value="active"
│   └── [PASS] Eq("active") | value="active"
└── [FAIL] Field("age") | value=15
    └── [FAIL] Gte(18) | value=15
```

## Semantics & Data Shapes

- **Conditions** accept any document shape: `mongory.M`, `map[string]any`, `bson.M`, `bson.D`, a struct, or `nil` (matches everything). Dot paths (`"address.city"`), escaped dots (`"a\\.b"`), array indexes (`"tags.0"`, `"tags.-1"`), and MongoDB implicit array matching apply.
- **Records** may be maps, `bson.M`/`bson.D`, structs or struct pointers, slices, or scalars. `ObjectID` (`[N]byte`) and `DateTime` (`Time() time.Time`) shapes are supported without importing a BSON driver.
- **Struct field keys** resolve tags in priority order (`mongory` → `bson` → `json` → bare tag without `key:`), using the first explicit name found and falling back to the exact Go field name (no lowercasing). Empty tag names (such as `",omitempty"`) and `"-"` tags yield no name and continue to the next tag or the Go field name (`json:"-,"` preserves literal `"-"`). Only `,inline` struct/map fields are flattened.
- **Operators**: `$eq $ne $gt $gte $lt $lte $in $nin $exists $present $and $or $not $elemMatch $every $size $regex` (with `$options`: `i`, `m`, `s`). Unknown `$` operators are rejected by `Compile`.

## Conformance & Benchmarks

Behavior is verified against the cross-language [mongory-spec](https://github.com/mongoryhq/mongory-spec) fixtures:

```bash
MONGORY_SPEC_DIR=../mongory-spec go test -v ./...
go test -run '^$' -bench . -benchmem
```

## Versioning Policy

- Use `v0.x` while the API is evolving
- Release `v1` once stable
- For `v2+`, use semantic import paths (e.g., `github.com/mongoryhq/mongory-go/v2`)

## License

License: MIT
