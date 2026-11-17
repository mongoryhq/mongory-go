package engine

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

const inSetThreshold = 16

func init() {
	register("$in", func(ctx *compileCtx, c any) (matcher, error) { return buildIn(ctx, "$in", c, false) })
	register("$nin", func(ctx *compileCtx, c any) (matcher, error) { return buildIn(ctx, "$nin", c, true) })
}

// inMatcher matches a value equal to any listed item; an array value matches
// when it shares at least one element with the list. $nin negates it, so a
// missing field matches $nin but never $in.
type inMatcher struct {
	items  []any
	set    *inLookup // populated when len(items) >= inSetThreshold
	negate bool
}

func buildIn(ctx *compileCtx, operator string, condition any, negate bool) (matcher, error) {
	items, ok := condition.([]any)
	if !ok {
		return nil, invalidCondition("%s needs an array, got %T", operator, condition)
	}
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	var set *inLookup
	if len(items) >= inSetThreshold {
		set = buildInLookup(items)
	}
	return inMatcher{items: items, set: set, negate: negate}, nil
}

func (m inMatcher) match(record any) bool {
	found := false
	if arr, ok := record.([]any); ok {
		for _, e := range arr {
			if found = m.contains(e); found {
				break
			}
		}
	} else {
		found = m.contains(record)
	}
	return found != m.negate
}

func (m inMatcher) matchReflect(rv reflect.Value) bool {
	found := false
	if list, ok := unwrapReflect(rv); ok && isReflectList(list) {
		for i := range list.Len() {
			if found = m.containsReflect(list.Index(i)); found {
				break
			}
		}
	} else {
		found = m.containsReflect(rv)
	}
	return found != m.negate
}

func (m inMatcher) contains(v any) bool {
	if m.set != nil {
		return m.set.contains(v)
	}
	for _, item := range m.items {
		if valuesEqual(v, item) {
			return true
		}
	}
	return false
}

func (m inMatcher) containsReflect(rv reflect.Value) bool {
	if m.set != nil {
		return m.set.containsReflect(rv)
	}
	for _, item := range m.items {
		if valuesEqualReflect(rv, item) {
			return true
		}
	}
	return false
}

func (m inMatcher) priority() int {
	if m.set != nil {
		return 3 + len(m.set.fallback)/8
	}
	return 2 + len(m.items)/8
}

func (m inMatcher) label() string {
	op := "In"
	if m.negate {
		op = "Nin"
	}
	return fmt.Sprintf("%s(%s)", op, formatValue(m.items))
}

func (m inMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}

func (m inMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}

// timeKey represents the UTC instant of a time.Time independent of Location or
// monotonic clock reading, matching time.Time.Equal.
type timeKey struct {
	sec  int64
	nsec int32
}

func makeTimeKey(t time.Time) timeKey {
	return timeKey{sec: t.Unix(), nsec: int32(t.Nanosecond())}
}

// inLookup provides O(1) hash lookup for large $in/$nin candidate lists while
// preserving numeric widening, timezone-agnostic time equality, and NaN rules.
type inLookup struct {
	hasNil   bool
	hasFalse bool
	hasTrue  bool
	strings  map[string]struct{}
	ints     map[int64]struct{}
	floats   map[float64]struct{}
	times    map[timeKey]struct{}
	other    map[any]struct{} // [N]byte opaque identifiers (ObjectID, UUID)
	fallback []any            // non-comparable containers (maps, slices, *regexp.Regexp)
}

func buildInLookup(items []any) *inLookup {
	s := &inLookup{}
	for _, item := range items {
		if norm, ok := normalizeFast(item); ok {
			item = norm
		} else {
			item = normalizeReflect(reflect.ValueOf(item))
		}
		switch x := item.(type) {
		case nil:
			s.hasNil = true
		case bool:
			if x {
				s.hasTrue = true
			} else {
				s.hasFalse = true
			}
		case string:
			if s.strings == nil {
				s.strings = make(map[string]struct{})
			}
			s.strings[x] = struct{}{}
		case int64:
			if s.ints == nil {
				s.ints = make(map[int64]struct{})
			}
			s.ints[x] = struct{}{}
		case float64:
			if math.IsNaN(x) {
				continue
			}
			if x == 0 {
				x = 0
			}
			if s.floats == nil {
				s.floats = make(map[float64]struct{})
			}
			s.floats[x] = struct{}{}
			// Pre-populate exact integral floats into ints so int64 lookups stay O(1)
			// without float rounding errors near 2^53.
			if x == math.Trunc(x) && x >= math.MinInt64 && x < math.MaxInt64 {
				if s.ints == nil {
					s.ints = make(map[int64]struct{})
				}
				s.ints[int64(x)] = struct{}{}
			}
		case time.Time:
			if s.times == nil {
				s.times = make(map[timeKey]struct{})
			}
			s.times[makeTimeKey(x)] = struct{}{}
		default:
			if rv := reflect.ValueOf(x); rv.IsValid() && rv.Kind() == reflect.Array && rv.Type().Elem().Kind() == reflect.Uint8 {
				if s.other == nil {
					s.other = make(map[any]struct{})
				}
				s.other[x] = struct{}{}
			} else {
				s.fallback = append(s.fallback, x)
			}
		}
	}
	return s
}

func (s *inLookup) hasBool(b bool) bool {
	if b {
		return s.hasTrue
	}
	return s.hasFalse
}

func (s *inLookup) hasString(v string) bool {
	if len(s.strings) == 0 {
		return false
	}
	_, ok := s.strings[v]
	return ok
}

func (s *inLookup) hasInt(v int64) bool {
	if len(s.ints) == 0 {
		return false
	}
	_, ok := s.ints[v]
	return ok
}

func (s *inLookup) hasFloat(f float64) bool {
	if math.IsNaN(f) {
		return false
	}
	if f == 0 {
		f = 0
	}
	if len(s.floats) > 0 {
		if _, ok := s.floats[f]; ok {
			return true
		}
	}
	if len(s.ints) > 0 && f == math.Trunc(f) && f >= math.MinInt64 && f < math.MaxInt64 {
		_, ok := s.ints[int64(f)]
		return ok
	}
	return false
}

func (s *inLookup) hasTime(t time.Time) bool {
	if len(s.times) == 0 {
		return false
	}
	_, ok := s.times[makeTimeKey(t)]
	return ok
}

func (s *inLookup) contains(v any) bool {
	if norm, ok := normalizeFast(v); ok {
		v = norm
	} else {
		return s.containsReflect(reflect.ValueOf(v))
	}
	switch x := v.(type) {
	case missingKey:
		return false
	case nil:
		return s.hasNil
	case bool:
		return s.hasBool(x)
	case string:
		return s.hasString(x)
	case int64:
		return s.hasInt(x)
	case float64:
		return s.hasFloat(x)
	case time.Time:
		return s.hasTime(x)
	default:
		for _, item := range s.fallback {
			if valuesEqual(v, item) {
				return true
			}
		}
		return false
	}
}

func (s *inLookup) containsReflect(rv reflect.Value) bool {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return s.hasNil
	}
	if t, ok := extractTime(rv); ok {
		return s.hasTime(t)
	}
	switch rv.Kind() {
	case reflect.Bool:
		return s.hasBool(rv.Bool())
	case reflect.String:
		if rv.Type() == jsonNumberType {
			switch x := parseJSONNumberString(rv.String()).(type) {
			case int64:
				return s.hasInt(x)
			case float64:
				return s.hasFloat(x)
			case string:
				return s.hasString(x)
			}
			return false
		}
		return s.hasString(rv.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return s.hasInt(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := rv.Uint(); u <= math.MaxInt64 {
			return s.hasInt(int64(u))
		}
		return s.hasFloat(float64(rv.Uint()))
	case reflect.Float32, reflect.Float64:
		return s.hasFloat(rv.Float())
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			if len(s.other) == 0 {
				return false
			}
			_, ok := s.other[rv.Interface()]
			return ok
		}
	}
	for _, item := range s.fallback {
		if valuesEqualReflect(rv, item) {
			return true
		}
	}
	return false
}
