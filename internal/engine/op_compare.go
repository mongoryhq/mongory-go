package engine

import (
	"fmt"
	"reflect"
	"strings"
)

func init() {
	register("$eq", func(c any) (matcher, error) { return eqMatcher{condition: c}, nil })
	register("$ne", func(c any) (matcher, error) { return neMatcher{condition: c}, nil })
	register("$gt", buildOrder("Gt", func(c int) bool { return c > 0 }))
	register("$gte", buildOrder("Gte", func(c int) bool { return c >= 0 }))
	register("$lt", buildOrder("Lt", func(c int) bool { return c < 0 }))
	register("$lte", buildOrder("Lte", func(c int) bool { return c <= 0 }))
}

// eqMatcher matches a value equal to the condition, or an array containing it.
type eqMatcher struct {
	condition any
}

func (m eqMatcher) match(record any) bool {
	if valuesEqual(record, m.condition) {
		return true
	}
	if arr, ok := record.([]any); ok {
		for _, e := range arr {
			if valuesEqual(e, m.condition) {
				return true
			}
		}
	}
	return false
}

func (m eqMatcher) matchReflect(rv reflect.Value) bool {
	if valuesEqualReflect(rv, m.condition) {
		return true
	}
	if list, ok := unwrapReflect(rv); ok && isReflectList(list) {
		for i := range list.Len() {
			if valuesEqualReflect(list.Index(i), m.condition) {
				return true
			}
		}
	}
	return false
}

func (eqMatcher) priority() int   { return 1 }
func (m eqMatcher) label() string { return fmt.Sprintf("Eq(%s)", formatValue(m.condition)) }
func (m eqMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m eqMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}

// neMatcher is the strict negation of eqMatcher.
type neMatcher struct {
	condition any
}

func (m neMatcher) match(record any) bool              { return !eqMatcher(m).match(record) }
func (m neMatcher) matchReflect(rv reflect.Value) bool { return !eqMatcher(m).matchReflect(rv) }
func (neMatcher) priority() int                        { return 1 }
func (m neMatcher) label() string                      { return fmt.Sprintf("Ne(%s)", formatValue(m.condition)) }
func (m neMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m neMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}

// orderMatcher implements $gt/$gte/$lt/$lte; incomparable pairs never match.
type orderMatcher struct {
	op        string
	condition any
	accept    func(cmp int) bool
}

func buildOrder(op string, accept func(cmp int) bool) builderFunc {
	return func(c any) (matcher, error) {
		return orderMatcher{op: op, condition: c, accept: accept}, nil
	}
}

func (m orderMatcher) match(record any) bool {
	cmp, ok := compareValues(record, m.condition)
	return ok && m.accept(cmp)
}

func (m orderMatcher) matchReflect(rv reflect.Value) bool {
	cmp, ok := compareReflect(rv, m.condition)
	return ok && m.accept(cmp)
}

func (orderMatcher) priority() int { return 3 }
func (m orderMatcher) label() string {
	return fmt.Sprintf("%s(%s)", m.op, formatValue(m.condition))
}
func (m orderMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m orderMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}
