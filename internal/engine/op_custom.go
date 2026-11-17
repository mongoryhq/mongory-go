package engine

import (
	"fmt"
	"reflect"
	"strings"
)

type customOpMatcher struct {
	name         string
	arg          any
	pred         Predicate
	priorityHint int
}

func buildCustomOp(ctx *compileCtx, name string, op Operator, arg any) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	pred, err := op.Compile(arg)
	if err != nil {
		return nil, invalidCondition("%s: %w", name, err)
	}
	if pred == nil {
		return nil, invalidCondition("%s: Compile returned nil Predicate", name)
	}
	prio := op.Priority
	if prio <= 0 {
		prio = 3
	}
	return customOpMatcher{
		name:         name,
		arg:          arg,
		pred:         pred,
		priorityHint: prio,
	}, nil
}

func (m customOpMatcher) match(record any) bool {
	if _, isMissing := record.(missingKey); isMissing {
		return m.pred(nil, false)
	}
	return m.pred(record, true)
}

func (m customOpMatcher) matchReflect(rv reflect.Value) bool {
	if !rv.IsValid() {
		return m.pred(nil, false)
	}
	if !rv.CanInterface() {
		return false
	}
	return m.pred(rv.Interface(), true)
}

func (m customOpMatcher) priority() int { return m.priorityHint }

func (m customOpMatcher) label() string {
	return fmt.Sprintf("%s(%s)", m.name, formatValue(m.arg))
}

func (m customOpMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}

func (m customOpMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	matched := m.match(record)
	line := fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), m.label(), formatTraceValue(record))
	writeTreeLine(buf, prefix, isLast, isRoot, line)
	return matched
}
