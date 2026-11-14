package engine

import (
	"math"
	"reflect"
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
