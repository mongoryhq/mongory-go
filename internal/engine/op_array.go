package engine

import (
	"fmt"
	"reflect"
	"strings"
)

func init() {
	register("$elemMatch", buildElemMatch)
	register("$every", buildEvery)
	register("$size", buildSize)
}

// elemMatchMatcher matches an array with at least one element satisfying the
// condition.
type elemMatchMatcher struct {
	priorityNode
	inner matcher
	raw   bool
}

func buildElemMatch(ctx *compileCtx, condition any) (matcher, error) {
	inner, err := buildElementCondition(ctx, "$elemMatch", condition)
	if err != nil {
		return nil, err
	}
	m := &elemMatchMatcher{inner: inner, raw: containsCustomOp(inner)}
	attachChild(inner, m)
	return m, nil
}

func matchListElementsAny(record any, inner matcher, raw, stopOn bool) bool {
	arr, ok := record.([]any)
	if !ok || len(arr) == 0 {
		return false
	}
	if raw {
		for _, e := range arr {
			if evalMatcherOnRaw(inner, e) == stopOn {
				return stopOn
			}
		}
		return !stopOn
	}
	for _, e := range arr {
		if matchRaw(inner, e) == stopOn {
			return stopOn
		}
	}
	return !stopOn
}

func matchListElementsReflect(rv reflect.Value, inner matcher, raw, stopOn bool) bool {
	list, ok := unwrapReflect(rv)
	if !ok || !isReflectList(list) || list.Len() == 0 {
		return false
	}
	if raw {
		for i := range list.Len() {
			if matchReflectValueRaw(inner, list.Index(i), true) == stopOn {
				return stopOn
			}
		}
		return !stopOn
	}
	for i := range list.Len() {
		if matchReflectValue(inner, list.Index(i)) == stopOn {
			return stopOn
		}
	}
	return !stopOn
}

func (m *elemMatchMatcher) match(record any) bool {
	return matchListElementsAny(record, m.inner, m.raw, true)
}

func (m *elemMatchMatcher) matchReflect(rv reflect.Value) bool {
	return matchListElementsReflect(rv, m.inner, m.raw, true)
}

func (m *elemMatchMatcher) priority() int { return 3 + m.inner.priority() }
func (m *elemMatchMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	explainBranch(buf, prefix, isLast, isRoot, "ElemMatch", m.inner)
}
func (m *elemMatchMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceArrayElements(record, buf, prefix, isLast, isRoot, "ElemMatch", m.inner, true)
}

// everyMatcher matches a non-empty array whose elements all satisfy the
// condition.
type everyMatcher struct {
	priorityNode
	inner matcher
	raw   bool
}

func buildEvery(ctx *compileCtx, condition any) (matcher, error) {
	inner, err := buildElementCondition(ctx, "$every", condition)
	if err != nil {
		return nil, err
	}
	m := &everyMatcher{inner: inner, raw: containsCustomOp(inner)}
	attachChild(inner, m)
	return m, nil
}

func (m *everyMatcher) match(record any) bool {
	return matchListElementsAny(record, m.inner, m.raw, false)
}

func (m *everyMatcher) matchReflect(rv reflect.Value) bool {
	return matchListElementsReflect(rv, m.inner, m.raw, false)
}

func (m *everyMatcher) priority() int { return 3 + m.inner.priority() }
func (m *everyMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	explainBranch(buf, prefix, isLast, isRoot, "Every", m.inner)
}
func (m *everyMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceArrayElements(record, buf, prefix, isLast, isRoot, "Every", m.inner, false)
}

func extractTraceList(record any) ([]any, bool) {
	if arr, ok := record.([]any); ok {
		return arr, true
	}
	if record == nil {
		return nil, false
	}
	list, ok := unwrapReflect(reflect.ValueOf(record))
	if !ok || !isReflectList(list) {
		return nil, false
	}
	out := make([]any, list.Len())
	for i := range list.Len() {
		out[i] = list.Index(i).Interface()
	}
	return out, true
}

func traceArrayElements(record any, buf *strings.Builder, prefix string, isLast, isRoot bool, op string, inner matcher, stopOn bool) bool {
	arr, ok := extractTraceList(record)
	if !ok || len(arr) == 0 {
		writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] %s | value=%s", op, formatTraceValue(record)))
		return false
	}
	childPrefix := childPrefixFor(prefix, isLast, isRoot)
	var childBuf strings.Builder
	matched := !stopOn
	for i, e := range arr {
		elemLast := i == len(arr)-1
		elemChildPrefix := childPrefixFor(childPrefix, elemLast, false)
		var elemBuf strings.Builder
		res := inner.trace(e, &elemBuf, elemChildPrefix, true, false)
		writeTreeLine(&childBuf, childPrefix, elemLast, false, fmt.Sprintf("[%s] [%d] | value=%s", statusLabel(res), i, formatTraceValue(e)))
		childBuf.WriteString(elemBuf.String())
		if res == stopOn {
			matched = stopOn
			if rem := len(arr) - i - 1; rem > 0 {
				writeTreeLine(&childBuf, childPrefix, true, false, fmt.Sprintf("[SKIP] %d remaining element(s) (short-circuited)", rem))
			}
			break
		}
	}
	writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), op, formatTraceValue(record)))
	buf.WriteString(childBuf.String())
	return matched
}

func buildElementCondition(ctx *compileCtx, operator string, condition any) (matcher, error) {
	c, ok := condition.(map[string]any)
	if !ok {
		return nil, invalidCondition("%s needs an object, got %T", operator, condition)
	}
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	return buildHashCondition(ctx, c)
}

// sizeMatcher matches an array whose length satisfies a literal condition,
// so both {"$size": 3} and {"$size": {"$gte": 1}} work.
type sizeMatcher struct {
	priorityNode
	inner matcher
}

func buildSize(ctx *compileCtx, condition any) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	inner, err := buildLiteral(ctx, condition)
	if err != nil {
		return nil, err
	}
	m := &sizeMatcher{inner: inner}
	attachChild(inner, m)
	return m, nil
}

func (m *sizeMatcher) match(record any) bool {
	arr, ok := record.([]any)
	return ok && m.inner.match(int64(len(arr)))
}

func (m *sizeMatcher) matchReflect(rv reflect.Value) bool {
	list, ok := unwrapReflect(rv)
	return ok && isReflectList(list) && m.inner.match(int64(list.Len()))
}

func (m *sizeMatcher) priority() int { return 2 + m.inner.priority() }
func (m *sizeMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	explainBranch(buf, prefix, isLast, isRoot, "Size", m.inner)
}
func (m *sizeMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	arr, ok := extractTraceList(record)
	if !ok {
		writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] Size | value=%s", formatTraceValue(record)))
		return false
	}
	return traceWrapped(int64(len(arr)), buf, prefix, isLast, isRoot, fmt.Sprintf("Size(len=%d)", len(arr)), m.inner, false)
}
