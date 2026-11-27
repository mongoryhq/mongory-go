package engine

import (
	"fmt"
	"reflect"
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

func (m *elemMatchMatcher) priority() int { return 8 + 2*m.inner.priority() }
func (m *elemMatchMatcher) explainNode() ExplainNode {
	return explainBranchNode("elem_match", "$elemMatch", "", "ElemMatch", m.priority(), m.inner)
}
func (m *elemMatchMatcher) traceNode(record any) (bool, TraceNode) {
	return traceArrayElementsNode(record, "elem_match", "$elemMatch", "ElemMatch", m.priority(), m.inner, true)
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

func (m *everyMatcher) priority() int { return 8 + 2*m.inner.priority() }
func (m *everyMatcher) explainNode() ExplainNode {
	return explainBranchNode("every", "$every", "", "Every", m.priority(), m.inner)
}
func (m *everyMatcher) traceNode(record any) (bool, TraceNode) {
	return traceArrayElementsNode(record, "every", "$every", "Every", m.priority(), m.inner, false)
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

func traceArrayElementsNode(record any, kind, operator, op string, priority int, inner matcher, stopOn bool) (bool, TraceNode) {
	arr, ok := extractTraceList(record)
	if !ok || len(arr) == 0 {
		return false, TraceNode{
			Kind:      kind,
			Status:    TraceFail,
			Label:     op,
			Priority:  priority,
			Operator:  operator,
			ValueText: formatTraceValue(record),
			Value:     snapshotTraceValue(record),
			Exists:    boolPtr(recordExists(record)),
		}
	}
	children := make([]TraceNode, 0, len(arr))
	matched := !stopOn
	for i, e := range arr {
		res, elemChild := inner.traceNode(e)
		children = append(children, TraceNode{
			Kind:      "element",
			Status:    traceStatus(res),
			Label:     fmt.Sprintf("[%d]", i),
			Index:     intPtr(i),
			ValueText: formatTraceValue(e),
			Value:     snapshotTraceValue(e),
			Exists:    boolPtr(recordExists(e)),
			Children:  []TraceNode{elemChild},
		})
		if res == stopOn {
			matched = stopOn
			if rem := len(arr) - i - 1; rem > 0 {
				children = append(children, TraceNode{
					Kind:         "skip",
					Status:       TraceSkip,
					Label:        fmt.Sprintf("%d remaining element(s) (short-circuited)", rem),
					SkippedCount: rem,
				})
			}
			break
		}
	}
	return matched, TraceNode{
		Kind:      kind,
		Status:    traceStatus(matched),
		Label:     op,
		Priority:  priority,
		Operator:  operator,
		ValueText: formatTraceValue(record),
		Value:     snapshotTraceValue(record),
		Exists:    boolPtr(recordExists(record)),
		Children:  children,
	}
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

func (m *sizeMatcher) priority() int { return 1 + m.inner.priority()/2 }
func (m *sizeMatcher) explainNode() ExplainNode {
	return explainBranchNode("size", "$size", "", "Size", m.priority(), m.inner)
}
func (m *sizeMatcher) traceNode(record any) (bool, TraceNode) {
	arr, ok := extractTraceList(record)
	if !ok {
		return false, TraceNode{
			Kind:      "size",
			Status:    TraceFail,
			Label:     "Size",
			Priority:  m.priority(),
			Operator:  "$size",
			ValueText: formatTraceValue(record),
			Value:     snapshotTraceValue(record),
			Exists:    boolPtr(recordExists(record)),
		}
	}
	return traceWrappedNode(int64(len(arr)), "size", "$size", "", fmt.Sprintf("Size(len=%d)", len(arr)), m.priority(), m.inner, false)
}
