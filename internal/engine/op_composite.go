package engine

import "reflect"

func init() {
	register("$and", func(ctx *compileCtx, c any) (matcher, error) {
		children, err := buildConditionList(ctx, "$and", c)
		if err != nil {
			return nil, err
		}
		flat := make([]matcher, 0, len(children))
		for _, child := range children {
			if sub, ok := child.(*allOf); ok {
				flat = append(flat, sub.slice()...)
				continue
			}
			flat = append(flat, child)
		}
		sortByPriority(flat)
		return newAllOf(flat), nil
	})
	register("$or", func(ctx *compileCtx, c any) (matcher, error) {
		children, err := buildConditionList(ctx, "$or", c)
		if err != nil {
			return nil, err
		}
		return newAnyOf(children), nil
	})
	register("$not", buildNot)
}

// buildConditionList builds one hash condition per element; an empty list
// yields an empty allOf (matches anything) or anyOf (matches nothing).
func buildConditionList(ctx *compileCtx, operator string, condition any) ([]matcher, error) {
	list, ok := condition.([]any)
	if !ok {
		return nil, invalidCondition("%s needs an array, got %T", operator, condition)
	}
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	children := make([]matcher, 0, len(list))
	for _, item := range list {
		sub, ok := item.(map[string]any)
		if !ok {
			return nil, invalidCondition("%s needs an array of objects, got %T", operator, item)
		}
		child, err := buildHashCondition(ctx, sub)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	sortByPriority(children)
	return children, nil
}

// notMatcher negates a literal condition, so it also matches missing fields.
type notMatcher struct {
	priorityNode
	inner matcher
}

func buildNot(ctx *compileCtx, condition any) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	inner, err := buildLiteral(ctx, condition)
	if err != nil {
		return nil, err
	}
	m := &notMatcher{inner: inner}
	attachChild(inner, m)
	return m, nil
}

func (m *notMatcher) match(record any) bool              { return !m.inner.match(record) }
func (m *notMatcher) matchReflect(rv reflect.Value) bool { return !m.inner.matchReflect(rv) }
func (m *notMatcher) priority() int                      { return 2 + m.inner.priority() }
func (m *notMatcher) explainNode() ExplainNode {
	return explainBranchNode("not", "$not", "", "Not", m.priority(), m.inner)
}
func (m *notMatcher) traceNode(record any) (bool, TraceNode) {
	return traceWrappedNode(record, "not", "$not", "", "Not", m.priority(), m.inner, true)
}
