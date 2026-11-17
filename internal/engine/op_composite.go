package engine

import (
	"reflect"
	"strings"
)

func init() {
	register("$and", func(ctx *compileCtx, c any) (matcher, error) {
		children, err := buildConditionList(ctx, "$and", c)
		return allOf(children), err
	})
	register("$or", func(ctx *compileCtx, c any) (matcher, error) {
		children, err := buildConditionList(ctx, "$or", c)
		return anyOf(children), err
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
	inner *literalMatcher
}

func buildNot(ctx *compileCtx, condition any) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	inner, err := buildLiteral(ctx, condition)
	if err != nil {
		return nil, err
	}
	return notMatcher{inner: inner}, nil
}

func (m notMatcher) match(record any) bool              { return !m.inner.match(record) }
func (m notMatcher) matchReflect(rv reflect.Value) bool { return !m.inner.matchReflect(rv) }
func (m notMatcher) priority() int                      { return 1 + m.inner.priority() }
func (m notMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	explainBranch(buf, prefix, isLast, isRoot, "Not", m.inner)
}
func (m notMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceWrapped(record, buf, prefix, isLast, isRoot, "Not", m.inner, true)
}
