# mongory-go

Zero-dependency, pure Go MongoDB-style query matcher for in-memory Go data: compile a filter condition once into an immutable, concurrent-safe `*mongory.Matcher` and evaluate maps, BSON documents, slices, or structs.

## Installation

```bash
go get github.com/mongoryhq/mongory-go
```

Requires Go 1.22 or later (`CGO_ENABLED=0` compatible).

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
	Age    int      `bson:"age"`
	Tags   []string `bson:"tags"`
	Active bool     // no tag: key defaults to "active"
}

var adults = mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})

func main() {
	users := []User{
		{Name: "Ann", Age: 30, Tags: []string{"go"}, Active: true},
		{Name: "Bob", Age: 15, Tags: []string{"ruby"}, Active: true},
		{Name: "Cid", Age: 42, Tags: []string{"go", "c"}},
	}

	m, err := mongory.Compile(mongory.M{
		"active": true,
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

### Debugging with `Explain` and `Trace`

Both `Explain()` and `Trace(record)` are pure, non-mutating, and safe to call concurrently with `Match`:

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
- **Records** may be maps, `bson.M`/`bson.D`, structs or struct pointers, slices, or scalars. Struct fields follow the mongo-go-driver convention: `mongory` tag override, then `bson` tag, else lowercased field name; only `,inline` fields are flattened. `ObjectID` (`[N]byte`) and `DateTime` (`Time() time.Time`) shapes are supported without importing a BSON driver.
- **Operators**: `$eq $ne $gt $gte $lt $lte $in $nin $exists $present $and $or $not $elemMatch $every $size $regex` (with `$options`: `i`, `m`, `s`). Unknown `$` operators are rejected by `Compile`.

## Conformance & Benchmarks

Behavior is verified against the cross-language [mongory-spec](https://github.com/mongoryhq/mongory-spec) fixtures:

```bash
MONGORY_SPEC_DIR=../mongory-spec CGO_ENABLED=0 go test -v ./...
go test -run '^$' -bench . -benchmem
```

## Versioning Policy

- Use `v0.x` while the API is evolving
- Release `v1` once stable
- For `v2+`, use semantic import paths (e.g., `github.com/mongoryhq/mongory-go/v2`)

## License

License: MIT
