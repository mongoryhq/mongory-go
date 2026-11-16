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
	if nb, ok := normalizeFast(b); ok {
		b = nb
	} else {
		b = normalizeReflect(reflect.ValueOf(b))
	}
	na, ok := normalizeFast(a)
	if !ok {
		return valuesEqualReflect(reflect.ValueOf(a), b)
	}
	a = na
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
		return intEqualsAny(x, b)
	case float64:
		return floatEqualsAny(x, b)
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

func intEqualsAny(x int64, b any) bool {
	switch y := b.(type) {
	case int64:
		return x == y
	case float64:
		return intEqualsFloat(x, y)
	}
	return false
}

func floatEqualsAny(x float64, b any) bool {
	switch y := b.(type) {
	case float64:
		return x == y
	case int64:
		return intEqualsFloat(y, x)
	}
	return false
}

// valuesEqualReflect compares an unnormalized reflect.Value against an already
// normalized canonical value b without allocating intermediate maps or []any.
func valuesEqualReflect(rv reflect.Value, b any) bool {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return b == nil
	}
	if t, ok := extractTime(rv); ok {
		y, ok := b.(time.Time)
		return ok && t.Equal(y)
	}
	switch rv.Kind() {
	case reflect.Bool:
		y, ok := b.(bool)
		return ok && rv.Bool() == y
	case reflect.String:
		if rv.Type() == jsonNumberType {
			return valuesEqual(parseJSONNumberString(rv.String()), b)
		}
		y, ok := b.(string)
		return ok && rv.String() == y
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEqualsAny(rv.Int(), b)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := rv.Uint(); u <= math.MaxInt64 {
			return intEqualsAny(int64(u), b)
		}
		return floatEqualsAny(float64(rv.Uint()), b)
	case reflect.Float32, reflect.Float64:
		return floatEqualsAny(rv.Float(), b)
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return reflect.DeepEqual(rv.Interface(), b)
		}
		return sliceEqualsReflect(rv, b)
	case reflect.Slice:
		if isKeyValueStruct(rv.Type().Elem()) {
			y, ok := b.(map[string]any)
			return ok && valuesEqual(orderedDocToMap(rv), y)
		}
		return sliceEqualsReflect(rv, b)
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return reflect.DeepEqual(rv.Interface(), b)
		}
		y, ok := b.(map[string]any)
		if !ok || rv.Len() != len(y) {
			return false
		}
		for k, yv := range y {
			ev := rv.MapIndex(reflect.ValueOf(k))
			if !ev.IsValid() || !valuesEqualReflect(ev, yv) {
				return false
			}
		}
		return true
	case reflect.Struct:
		if isRegexStruct(rv.Type()) {
			return valuesEqual(normalizeReflect(rv), b)
		}
		y, ok := b.(map[string]any)
		return ok && valuesEqual(structToMap(rv), y)
	}
	return reflect.DeepEqual(rv.Interface(), b)
}

func sliceEqualsReflect(rv reflect.Value, b any) bool {
	y, ok := b.([]any)
	if !ok || rv.Len() != len(y) {
		return false
	}
	for i := range y {
		if !valuesEqualReflect(rv.Index(i), y[i]) {
			return false
		}
	}
	return true
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
	if nb, okB := normalizeFast(b); okB {
		b = nb
	} else {
		b = normalizeReflect(reflect.ValueOf(b))
	}
	na, okA := normalizeFast(a)
	if !okA {
		return compareReflect(reflect.ValueOf(a), b)
	}
	a = na
	switch x := a.(type) {
	case int64:
		return compareIntToAny(x, b)
	case float64:
		return compareFloatToAny(x, b)
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

func compareIntToAny(x int64, b any) (int, bool) {
	switch y := b.(type) {
	case int64:
		return cmpOrdered(x, y), true
	case float64:
		if math.IsNaN(y) {
			return 0, false
		}
		return compareIntFloat(x, y), true
	}
	return 0, false
}

func compareFloatToAny(x float64, b any) (int, bool) {
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
	return 0, false
}

func compareReflect(rv reflect.Value, b any) (int, bool) {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return 0, false
	}
	if t, ok := extractTime(rv); ok {
		if y, ok := b.(time.Time); ok {
			return t.Compare(y), true
		}
		return 0, false
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return compareIntToAny(rv.Int(), b)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := rv.Uint(); u <= math.MaxInt64 {
			return compareIntToAny(int64(u), b)
		}
		return compareFloatToAny(float64(rv.Uint()), b)
	case reflect.Float32, reflect.Float64:
		return compareFloatToAny(rv.Float(), b)
	case reflect.String:
		if rv.Type() == jsonNumberType {
			return compareValues(parseJSONNumberString(rv.String()), b)
		}
		if y, ok := b.(string); ok {
			return strings.Compare(rv.String(), y), true
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
