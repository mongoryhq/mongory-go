package engine

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

// convValueMatcher wraps a scalar built-in leaf matcher ($gt, $gte, $lt, $lte,
// $regex, $present) so registered converters transform incoming record values
// while leaving default (no-converter) matcher structs zero-overhead.
type convValueMatcher struct {
	conv  *converterRegistry
	inner matcher
}

func (m convValueMatcher) match(record any) bool {
	if converted, ok := m.conv.applyNormalized(record); ok {
		return m.inner.match(converted)
	}
	return m.inner.match(record)
}

func (m convValueMatcher) matchReflect(rv reflect.Value) bool {
	if converted, ok := m.conv.applyReflectNormalized(rv); ok {
		return m.inner.match(converted)
	}
	return m.inner.matchReflect(rv)
}

func (m convValueMatcher) priority() int { return m.inner.priority() }
func (m convValueMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	m.inner.explain(buf, prefix, isLast, isRoot)
}
func (m convValueMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	matched := matchRaw(m, record)
	var labelBuf strings.Builder
	m.inner.explain(&labelBuf, "", true, true)
	label := strings.TrimRight(labelBuf.String(), "\n")
	line := fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), label, formatTraceValue(record))
	writeTreeLine(buf, prefix, isLast, isRoot, line)
	return matched
}

// eqConvMatcher implements $eq and $ne when converters are registered.
type eqConvMatcher struct {
	condition any
	conv      *converterRegistry
	negate    bool
}

func (m eqConvMatcher) matchEq(record any) bool {
	if valuesEqualConv(record, m.condition, m.conv) {
		return true
	}
	if arr, ok := record.([]any); ok {
		for _, e := range arr {
			if valuesEqualConv(e, m.condition, m.conv) {
				return true
			}
		}
	}
	return false
}

func (m eqConvMatcher) matchEqReflect(rv reflect.Value) bool {
	if valuesEqualReflectConv(rv, m.condition, m.conv) {
		return true
	}
	if list, ok := unwrapReflect(rv); ok && isReflectList(list) {
		for i := range list.Len() {
			if valuesEqualReflectConv(list.Index(i), m.condition, m.conv) {
				return true
			}
		}
	}
	return false
}

func (m eqConvMatcher) match(record any) bool              { return m.matchEq(record) != m.negate }
func (m eqConvMatcher) matchReflect(rv reflect.Value) bool { return m.matchEqReflect(rv) != m.negate }
func (eqConvMatcher) priority() int                        { return 1 }
func (m eqConvMatcher) label() string {
	op := "Eq"
	if m.negate {
		op = "Ne"
	}
	return fmt.Sprintf("%s(%s)", op, formatValue(m.condition))
}
func (m eqConvMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m eqConvMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}

// inConvMatcher implements $in and $nin when converters are registered.
type inConvMatcher struct {
	in   inMatcher
	conv *converterRegistry
}

func (m inConvMatcher) match(record any) bool {
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
	return found != m.in.negate
}

func (m inConvMatcher) matchReflect(rv reflect.Value) bool {
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
	return found != m.in.negate
}

func (m inConvMatcher) contains(v any) bool {
	if converted, ok := m.conv.applyNormalized(v); ok {
		return m.in.contains(converted)
	}
	if m.in.set != nil {
		return m.containsSetConv(v)
	}
	for _, item := range m.in.items {
		if valuesEqualConv(v, item, m.conv) {
			return true
		}
	}
	return false
}

func (m inConvMatcher) containsReflect(rv reflect.Value) bool {
	if converted, ok := m.conv.applyReflectNormalized(rv); ok {
		return m.in.contains(converted)
	}
	if m.in.set != nil {
		return m.containsSetReflectConv(rv)
	}
	for _, item := range m.in.items {
		if valuesEqualReflectConv(rv, item, m.conv) {
			return true
		}
	}
	return false
}

func (m inConvMatcher) containsSetConv(v any) bool {
	if norm, ok := normalizeFast(v); ok {
		if _, isMap := norm.(map[string]any); !isMap {
			if _, isSlice := norm.([]any); !isSlice {
				return m.in.set.contains(norm)
			}
		}
		for _, item := range m.in.set.fallback {
			if valuesEqualConv(norm, item, m.conv) {
				return true
			}
		}
		return false
	}
	return m.containsSetReflectConv(reflect.ValueOf(v))
}

func (m inConvMatcher) containsSetReflectConv(rv reflect.Value) bool {
	unwrapped, ok := unwrapReflect(rv)
	if !ok {
		return m.in.set.hasNil
	}
	if t, ok := extractTime(unwrapped); ok {
		return m.in.set.hasTime(t)
	}
	switch unwrapped.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return m.in.set.containsReflect(unwrapped)
	case reflect.Array:
		if unwrapped.Type().Elem().Kind() == reflect.Uint8 {
			return m.in.set.containsReflect(unwrapped)
		}
	}
	for _, item := range m.in.set.fallback {
		if valuesEqualReflectConv(rv, item, m.conv) {
			return true
		}
	}
	return false
}

func (m inConvMatcher) priority() int { return m.in.priority() }
func (m inConvMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	m.in.explain(buf, prefix, isLast, isRoot)
}
func (m inConvMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.in.label())
}

func valuesEqualConv(a, b any, conv *converterRegistry) bool {
	if nb, ok := normalizeFast(b); ok {
		b = nb
	} else {
		b = normalizeReflect(reflect.ValueOf(b))
	}
	if conv != nil {
		if converted, ok := conv.applyNormalized(a); ok {
			return valuesEqual(converted, b)
		}
	}
	na, ok := normalizeFast(a)
	if !ok {
		return valuesEqualReflectConv(reflect.ValueOf(a), b, conv)
	}
	switch x := na.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, ok := y[k]
			if !ok || !valuesEqualConv(xv, yv, conv) {
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
			if !valuesEqualConv(x[i], y[i], conv) {
				return false
			}
		}
		return true
	}
	return valuesEqual(na, b)
}

func valuesEqualReflectConv(rv reflect.Value, b any, conv *converterRegistry) bool {
	if conv != nil {
		if converted, ok := conv.applyReflectNormalized(rv); ok {
			return valuesEqual(converted, b)
		}
	}
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
		return sliceEqualsReflectConv(rv, b, conv)
	case reflect.Slice:
		if isKeyValueStruct(rv.Type().Elem()) {
			y, ok := b.(map[string]any)
			return ok && valuesEqualConv(orderedDocToMap(rv), y, conv)
		}
		return sliceEqualsReflectConv(rv, b, conv)
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
			if !ev.IsValid() || !valuesEqualReflectConv(ev, yv, conv) {
				return false
			}
		}
		return true
	case reflect.Struct:
		if isRegexStruct(rv.Type()) {
			return valuesEqual(normalizeReflect(rv), b)
		}
		y, ok := b.(map[string]any)
		return ok && valuesEqualConv(structToMap(rv), y, conv)
	}
	return reflect.DeepEqual(rv.Interface(), b)
}

func sliceEqualsReflectConv(rv reflect.Value, b any, conv *converterRegistry) bool {
	y, ok := b.([]any)
	if !ok || rv.Len() != len(y) {
		return false
	}
	for i := range y {
		if !valuesEqualReflectConv(rv.Index(i), y[i], conv) {
			return false
		}
	}
	return true
}
