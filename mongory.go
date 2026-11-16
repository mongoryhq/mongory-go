// Package mongory compiles MongoDB-style query filters and evaluates them
// against in-memory Go maps, BSON documents, slices, and structs.
//
// Compile a filter once with [Compile] (or [MustCompile]) and reuse the
// resulting [Matcher] safely across goroutines:
//
//	m := mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})
//	matched := mongory.Filter(users, m)
//
// For one-off checks, use [Match]. For debugging why a record matched or
// failed, inspect the compiled tree with [Matcher.Explain] or trace an
// evaluation step by step with [Matcher.Trace].
package mongory

import "github.com/mongoryhq/mongory-go/internal/engine"

// M and A are type aliases (not defined types), so they interoperate with
// map[string]any, []any, bson.M, and bson.A at zero cost.
type (
	M = map[string]any
	A = []any
)

// ErrInvalidCondition is wrapped by every error returned from Compile or Match
// for a malformed condition (unknown operator, invalid argument type, or bad
// $regex / $options); check with errors.Is.
var ErrInvalidCondition = engine.ErrInvalidCondition

// Compile parses and validates condition once into an immutable, concurrent-safe
// *Matcher. Like the mongo-go-driver filter parameter, condition accepts any
// document shape: mongory.M, map[string]any, bson.M, bson.D, a struct, or nil
// (which matches everything).
func Compile(condition any) (*Matcher, error) {
	tree, err := engine.Compile(condition)
	if err != nil {
		return nil, err
	}
	return &Matcher{tree: tree}, nil
}

// MustCompile is like Compile but panics on an invalid condition. It is meant
// for package-level or constant conditions fixed in source code, where a bad
// filter is a programmer bug that should fail at startup:
//
//	var activeUsers = mongory.MustCompile(mongory.M{"status": "active"})
//
// Never pass untrusted or runtime input to MustCompile; use Compile instead.
func MustCompile(condition any) *Matcher {
	m, err := Compile(condition)
	if err != nil {
		panic(err)
	}
	return m
}

// Match compiles condition and reports whether record satisfies it.
// For repeated checks against many records, compile once with Compile and reuse
// (*Matcher).Match or Filter.
func Match(condition any, record any) (bool, error) {
	m, err := Compile(condition)
	if err != nil {
		return false, err
	}
	return m.Match(record), nil
}
