package mongory

import "github.com/mongoryhq/mongory-go/internal/engine"

// M and A are aliases (not defined types), so they interoperate with
// map[string]any and []any at zero cost.
type (
	M = map[string]any
	A = []any
)

// ErrInvalidCondition is wrapped by every error returned from New for a
// malformed condition; test with errors.Is.
var ErrInvalidCondition = engine.ErrInvalidCondition

// Query is a compiled condition. It is safe for concurrent use.
type Query struct {
	tree *engine.Tree
}

// New compiles condition into a matcher tree once; Match then only walks it.
// Like the mongo-go-driver's filter argument, condition is any document:
// M, bson.M, bson.D, a struct, or nil (matches everything).
//
// Use New for conditions built at runtime (request input, config, database)
// and handle the error; use MustNew for conditions fixed in source code.
func New(condition any) (*Query, error) {
	tree, err := engine.Compile(condition)
	if err != nil {
		return nil, err
	}
	return &Query{tree: tree}, nil
}

// MustNew is like New but panics on an invalid condition. It is meant for
// conditions fixed in source code, such as package-level variables, where an
// invalid condition is a programming error that should fail at startup:
//
//	var activeUsers = mongory.MustNew(bson.M{"status": "active"})
//
// Never pass runtime input (request bodies, config files) to MustNew: a bad
// filter would crash the process instead of returning an error. Use New.
func MustNew(condition any) *Query {
	q, err := New(condition)
	if err != nil {
		panic(err)
	}
	return q
}

// Match reports whether record satisfies the query. record may be a map,
// bson.D, slice, struct (or pointer to one), or scalar. Struct fields map to
// keys as the mongo-go-driver does: the mongory or bson tag, else the
// lowercased field name; only ",inline" fields are flattened.
func (q *Query) Match(record any) bool {
	return q.tree.Match(record)
}
