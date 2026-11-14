# mongory-go

MongoDB-style query matching for in-memory Go data: compile a filter once, match any number of maps, `bson` documents or structs.

## Installation

```bash
go get github.com/mongoryhq/mongory-go
```

## Usage

A condition is compiled once with `New` (or `MustNew` for conditions fixed in
source code), then matched against any number of records.

```go
import "github.com/mongoryhq/mongory-go"

type User struct {
	Name   string   `bson:"name"`
	Age    int      `bson:"age"`
	Tags   []string `bson:"tags"`
	Active bool     // no tag: key is "active", as in the mongo-go-driver
}

// Fixed in source: an invalid condition panics at startup.
var adults = mongory.MustNew(mongory.M{"age": mongory.M{"$gte": 18}})

// Built at runtime: handle the error.
q, err := mongory.New(mongory.M{
	"active": true,
	"$or": mongory.A{
		mongory.M{"tags": "go"},
		mongory.M{"age": mongory.M{"$lt": 18}},
	},
})
if err != nil {
	return err // errors.Is(err, mongory.ErrInvalidCondition)
}

matched := mongory.Filter(users, q) // []User, input order kept
ok := adults.Match(users[0])         // single record
```

- **Conditions** accept the same shapes as a mongo-go-driver filter:
  `mongory.M`, `bson.M`, `bson.D`, a struct, or `nil` (match all).
  Dot paths (`"address.city"`), array indexes (`"tags.0"`) and MongoDB array
  semantics apply.
- **Records** may be maps, `bson.M`/`bson.D`, structs or struct pointers,
  slices, or scalars. Struct fields map to keys as the mongo-go-driver does:
  the `bson` tag (or a `mongory` tag override), else the lowercased field
  name; only `,inline` fields are flattened. `ObjectID` and `DateTime` values
  compare as expected.
- **Operators**: `$eq $ne $gt $gte $lt $lte $in $nin $exists $present $and $or
  $not $elemMatch $every $size $regex` (with `$options`: `i`, `m`, `s`).
  Unknown `$` operators are rejected by `New`.

A `Query` is safe for concurrent use. Behavior is verified against the
cross-language [mongory-spec](https://github.com/mongoryhq/mongory-spec)
fixtures:

```bash
MONGORY_SPEC_DIR=../mongory-spec go test ./...
```

> The legacy CGO API (`NewCMatcher`) remains available until the pure Go
> engine replaces it.

## Dependencies and System Requirements

- Go 1.24
- If/when CGO integration with `mongory-core` is enabled, a C toolchain may be required (e.g., clang/llvm, make).

## Versioning Policy

- Use v0.x while the API is unstable
- Release v1 once stable
- For v2+, use semantic import paths (e.g., `github.com/mongoryhq/mongory-go/v2`)

## License

License: MIT

