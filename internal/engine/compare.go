package engine

import (
	"math"
	"reflect"
	"strings"
	"time"
)

// valuesEqual is deep equality with int/float widening; map equality ignores
// key order, slice equality is order sensitive. A missing field equals nothing.
func valuesEqual(a, b any) bool {
	a, b = normalize(a), normalize(b)
	switch x := a.(type) {
	case nil:
		return b == nil
	case missingKey:
		return false
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case int64:
		switch y := b.(type) {
		case int64:
			return x == y
		case float64:
			return intEqualsFloat(x, y)
		}
		return false
	case float64:
		switch y := b.(type) {
		case float64:
			return x == y
		case int64:
			return intEqualsFloat(y, x)
		}
		return false
	case time.Time:
		y, ok := b.(time.Time)
		return ok && x.Equal(y)
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, ok := y[k]
			if !ok || !valuesEqual(xv, yv) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !valuesEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a, b)
}

// intEqualsFloat compares exactly, without rounding i through float64.
func intEqualsFloat(i int64, f float64) bool {
	if f != math.Trunc(f) || f < math.MinInt64 || f >= math.MaxInt64 {
		return false
	}
	return int64(f) == i
}

// compareValues orders a against b. ok is false when the pair is not
// comparable: only number/number, string/string (byte order) and time/time
// are, and NaN compares with nothing.
func compareValues(a, b any) (cmp int, ok bool) {
	a, b = normalize(a), normalize(b)
	switch x := a.(type) {
	case int64:
		switch y := b.(type) {
		case int64:
			return cmpOrdered(x, y), true
		case float64:
			if math.IsNaN(y) {
				return 0, false
			}
			return compareIntFloat(x, y), true
		}
	case float64:
		if math.IsNaN(x) {
			return 0, false
		}
		switch y := b.(type) {
		case float64:
			if math.IsNaN(y) {
				return 0, false
			}
			return cmpOrdered(x, y), true
		case int64:
			return -compareIntFloat(y, x), true
		}
	case string:
		if y, ok := b.(string); ok {
			return strings.Compare(x, y), true
		}
	case time.Time:
		if y, ok := b.(time.Time); ok {
			return x.Compare(y), true
		}
	}
	return 0, false
}

func cmpOrdered[T int64 | float64](x, y T) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// compareIntFloat compares exactly, without rounding i through float64.
// f must not be NaN.
func compareIntFloat(i int64, f float64) int {
	if f >= math.MaxInt64 {
		return -1
	}
	if f < math.MinInt64 {
		return 1
	}
	t := math.Trunc(f)
	if c := cmpOrdered(i, int64(t)); c != 0 {
		return c
	}
	return cmpOrdered(t, f)
}
