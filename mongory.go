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

const (
	// DefaultMaxDepth caps condition nesting depth during compilation.
	DefaultMaxDepth = engine.DefaultMaxDepth
	// DefaultMaxNodes caps the number of compiled matcher nodes.
	DefaultMaxNodes = engine.DefaultMaxNodes
)

// ErrInvalidCondition is wrapped by every error returned from Compile or Match
// for a malformed condition (unknown operator, invalid argument type, bad
// $regex / $options, or exceeded limits); check with errors.Is.
var ErrInvalidCondition = engine.ErrInvalidCondition

// ErrLimitExceeded is wrapped alongside ErrInvalidCondition when a condition
// exceeds the configured MaxDepth or MaxNodes limit.
var ErrLimitExceeded = engine.ErrLimitExceeded

// ErrInvalidOption is returned by Compile (and panicked on by MustCompile) when
// an Option has an invalid configuration such as a nil callback, malformed
// operator name, built-in operator override, or unsupported target type.
// It does not wrap ErrInvalidCondition.
var ErrInvalidOption = engine.ErrInvalidOption

// Option configures Compile, MustCompile, and Match behavior.
type Option = engine.Option

// Predicate evaluates a custom operator against raw record data.
// When exists is false, the field is missing from its parent container and
// value is nil; when exists is true, value retains its raw (including typed nil)
// representation.
type Predicate = engine.Predicate

// Operator defines a per-Compile custom query operator whose name starts with "$".
type Operator = engine.Operator

// TraceStatus is the evaluation outcome of one node in a [TraceNode] tree.
type TraceStatus = engine.TraceStatus

const (
	TracePass TraceStatus = engine.TracePass
	TraceFail TraceStatus = engine.TraceFail
	TraceSkip TraceStatus = engine.TraceSkip
)

// ExplainNode is one node in the structured tree returned by [Matcher.ExplainTree].
type ExplainNode = engine.ExplainNode

// TraceNode is one node in the structured evaluation trace returned by [Matcher.TraceTree].
type TraceNode = engine.TraceNode

// WithOptions bundles multiple options into one reusable preset Option applied
// left to right (later entries override earlier duplicates). Nil entries are
// ignored.
func WithOptions(opts ...Option) Option { return engine.WithOptions(opts...) }

// WithMaxDepth overrides DefaultMaxDepth (64) during compilation.
// A non-positive value disables the depth limit.
func WithMaxDepth(depth int) Option { return engine.WithMaxDepth(depth) }

// WithMaxNodes overrides DefaultMaxNodes (4096) during compilation.
// A non-positive value disables the node limit.
func WithMaxNodes(nodes int) Option { return engine.WithMaxNodes(nodes) }

// WithConverter registers a symmetric converter for values of type T across
// built-in operators and comparisons (on both conditions and records). Nil
// pointers become null before fn runs. Custom operators registered with
// [WithOperator] still receive raw unconverted values.
func WithConverter[T any](fn func(T) any) Option { return engine.WithTypeConverter[T](fn) }

// WithField binds a static getter function for a single field segment on values
// matching T (concrete type, pointer *T, or interface T).
func WithField[T any](name string, get func(T) any) Option { return engine.WithField[T](name, get) }

// WithFieldResolver registers a dynamic per-segment resolver for values matching
// T (concrete type, pointer *T, or interface T). Returning ok=false falls back
// to earlier resolvers and then struct tag / map lookup.
func WithFieldResolver[T any](fn func(rec T, field string) (val any, ok bool)) Option {
	return engine.WithFieldResolver[T](fn)
}

// WithOperator registers a per-Compile custom operator (for example "$moneyGte").
// Built-in operators and "$regex"/"$options" cannot be overridden.
func WithOperator(name string, op Operator) Option { return engine.WithOperator(name, op) }

// And combines multiple query maps into a normalized conjunction without mutating
// callers' maps. Nil and empty M{} entries are omitted as identity elements;
// nested $and lists are flattened, compatible pure-$ operator subdocuments are
// merged safely, and key conflicts are preserved inside $and.
func And(conds ...M) M { return engine.And(conds...) }

// Or combines multiple query maps into a normalized disjunction without mutating
// callers' maps. Nil (including typed-nil M(nil)) entries are skipped as omitted
// optional branches, while explicit non-nil M{} absorbs the disjunction to M{}
// per MongoDB match-all semantics. If no non-nil clauses remain, Or returns
// M{"$or": A{}} (fail-closed).
func Or(conds ...M) M { return engine.Or(conds...) }

// Compile parses and validates condition once into an immutable, concurrent-safe
// *Matcher. Like the mongo-go-driver filter parameter, condition accepts any
// document shape: mongory.M, map[string]any, bson.M, bson.D, a struct, or nil
// (which matches everything).
func Compile(condition any, opts ...Option) (*Matcher, error) {
	tree, err := engine.Compile(condition, opts...)
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
func MustCompile(condition any, opts ...Option) *Matcher {
	m, err := Compile(condition, opts...)
	if err != nil {
		panic(err)
	}
	return m
}

// Match compiles condition and reports whether record satisfies it.
// For repeated checks against many records, compile once with Compile and reuse
// (*Matcher).Match or Filter.
func Match(condition any, record any, opts ...Option) (bool, error) {
	m, err := Compile(condition, opts...)
	if err != nil {
		return false, err
	}
	return m.Match(record), nil
}
