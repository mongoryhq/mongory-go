package mongory

import (
	"reflect"

	"github.com/mongoryhq/mongory-go/internal/engine"
)

// Matcher is a compiled query condition. It is safe for concurrent use by
// multiple goroutines, including concurrent Match, Explain, and Trace calls.
type Matcher struct {
	tree *engine.Tree
}

// Match reports whether record satisfies the compiled condition. record may be
// a map, bson.M, bson.D, slice, struct (or pointer to one), or scalar. Struct
// fields resolve via mongory, bson, or json tag, falling back to the exact Go
// field name (including when tagged "-"); only ",inline" fields are flattened.
func (m *Matcher) Match(record any) bool {
	return m.tree.Match(record)
}

// Explain returns a human-readable tree diagram of the compiled matcher.
func (m *Matcher) Explain() string {
	return m.tree.Explain()
}

// String implements fmt.Stringer using Explain.
func (m *Matcher) String() string {
	return m.Explain()
}

// Trace evaluates record against the compiled condition and returns both the
// match outcome and a human-readable per-node evaluation trace.
// Trace does not mutate m and is safe to call concurrently with Match.
func (m *Matcher) Trace(record any) (bool, string) {
	return m.tree.Trace(record)
}

func isStructElem[E any]() bool {
	return reflect.TypeFor[E]().Kind() == reflect.Struct
}

// Filter returns a new slice containing every record in records that matches m,
// preserving order. The input slice is not modified. Returns nil when nothing
// matches.
func Filter[S ~[]E, E any](records S, m *Matcher) S {
	var out S
	if isStructElem[E]() {
		for i := range records {
			if m.Match(&records[i]) {
				out = append(out, records[i])
			}
		}
		return out
	}
	for _, r := range records {
		if m.Match(r) {
			out = append(out, r)
		}
	}
	return out
}

// Find returns the first record in records that matches m and reports whether
// one was found.
func Find[S ~[]E, E any](records S, m *Matcher) (E, bool) {
	if isStructElem[E]() {
		for i := range records {
			if m.Match(&records[i]) {
				return records[i], true
			}
		}
		var zero E
		return zero, false
	}
	for _, r := range records {
		if m.Match(r) {
			return r, true
		}
	}
	var zero E
	return zero, false
}

// Any reports whether at least one record in records matches m.
func Any[S ~[]E, E any](records S, m *Matcher) bool {
	_, ok := Find(records, m)
	return ok
}

// Count returns the number of records in records that match m.
func Count[S ~[]E, E any](records S, m *Matcher) int {
	n := 0
	if isStructElem[E]() {
		for i := range records {
			if m.Match(&records[i]) {
				n++
			}
		}
		return n
	}
	for _, r := range records {
		if m.Match(r) {
			n++
		}
	}
	return n
}
