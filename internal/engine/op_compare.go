package engine

import (
	"fmt"
	"reflect"
	"strings"
)

func init() {
	register("$eq", func(ctx *compileCtx, c any) (matcher, error) {
		if conv := ctx.convOrNil(); conv != nil {
			return addNodeMatcher(ctx, eqConvMatcher{condition: c, conv: conv})
		}
		return addNodeMatcher(ctx, eqMatcher{condition: c})
	})
	register("$ne", func(ctx *compileCtx, c any) (matcher, error) {
		if conv := ctx.convOrNil(); conv != nil {
			return addNodeMatcher(ctx, eqConvMatcher{condition: c, conv: conv, negate: true})
		}
		return addNodeMatcher(ctx, neMatcher{condition: c})
	})
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
func (m eqMatcher) explainNode() ExplainNode {
	return ExplainNode{Kind: "operator", Label: m.label(), Priority: m.priority(), Operator: "$eq"}
}
func (m eqMatcher) traceNode(record any) (bool, TraceNode) {
	return traceLeafNode(m, record, "operator", "$eq", m.label())
}

// neMatcher is the strict negation of eqMatcher.
type neMatcher struct {
	condition any
}

func (m neMatcher) match(record any) bool              { return !eqMatcher(m).match(record) }
func (m neMatcher) matchReflect(rv reflect.Value) bool { return !eqMatcher(m).matchReflect(rv) }
func (neMatcher) priority() int                        { return 1 }
func (m neMatcher) label() string                      { return fmt.Sprintf("Ne(%s)", formatValue(m.condition)) }
func (m neMatcher) explainNode() ExplainNode {
	return ExplainNode{Kind: "operator", Label: m.label(), Priority: m.priority(), Operator: "$ne"}
}
func (m neMatcher) traceNode(record any) (bool, TraceNode) {
	return traceLeafNode(m, record, "operator", "$ne", m.label())
}

// orderMatcher implements $gt/$gte/$lt/$lte; incomparable pairs never match.
type orderMatcher struct {
	op        string
	condition any
	accept    func(cmp int) bool
}

func buildOrder(op string, accept func(cmp int) bool) builderFunc {
	return func(ctx *compileCtx, c any) (matcher, error) {
		m := orderMatcher{op: op, condition: c, accept: accept}
		if conv := ctx.convOrNil(); conv != nil {
			return addNodeMatcher(ctx, convValueMatcher{conv: conv, inner: m})
		}
		return addNodeMatcher(ctx, m)
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
func (m orderMatcher) explainNode() ExplainNode {
	return ExplainNode{Kind: "operator", Label: m.label(), Priority: m.priority(), Operator: "$" + strings.ToLower(m.op)}
}
func (m orderMatcher) traceNode(record any) (bool, TraceNode) {
	return traceLeafNode(m, record, "operator", "$"+strings.ToLower(m.op), m.label())
}
