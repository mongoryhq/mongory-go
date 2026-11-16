package engine

import (
	"fmt"
	"reflect"
	"strings"
)

func init() {
	register("$in", func(c any) (matcher, error) { return buildIn("$in", c, false) })
	register("$nin", func(c any) (matcher, error) { return buildIn("$nin", c, true) })
}

// inMatcher matches a value equal to any listed item; an array value matches
// when it shares at least one element with the list. $nin negates it, so a
// missing field matches $nin but never $in.
type inMatcher struct {
	items  []any
	negate bool
}

func buildIn(operator string, condition any, negate bool) (matcher, error) {
	items, ok := condition.([]any)
	if !ok {
		return nil, invalidCondition("%s needs an array, got %T", operator, condition)
	}
	return inMatcher{items: items, negate: negate}, nil
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
	for _, item := range m.items {
		if valuesEqual(v, item) {
			return true
		}
	}
	return false
}

func (m inMatcher) containsReflect(rv reflect.Value) bool {
	for _, item := range m.items {
		if valuesEqualReflect(rv, item) {
			return true
		}
	}
	return false
}

func (m inMatcher) priority() int { return 2 + len(m.items)/8 }

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
