package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>

*/
import "C"
import "unsafe"

type Matcher struct {
	CPoint       *C.mongory_matcher
	condition    *map[string]any
	context      *any
	pool         *MemoryPool
	scratchPool  *MemoryPool
	traceEnabled bool
}

func NewMatcher(pool *MemoryPool, condition map[string]any, context *any) *Matcher {
	conditionValue := pool.ValueConvert(condition)
	scratchPool := NewMemoryPool()
	return &Matcher{
		CPoint:       C.mongory_matcher_new(pool.CPoint, conditionValue.CPoint, unsafe.Pointer(context)),
		condition:    &condition,
		context:      context,
		pool:         pool,
		scratchPool:  scratchPool,
		traceEnabled: false,
	}
}

func (m *Matcher) Match(value any) bool {
	convertedValue := m.scratchPool.ValueConvert(value)
	result := bool(C.mongory_matcher_match(m.CPoint, convertedValue.CPoint))
	if !m.traceEnabled {
		m.scratchPool.Reset()
	}
	return result
}

func (m *Matcher) Explain() {
	C.mongory_matcher_explain(m.CPoint, m.scratchPool.CPoint)
	m.scratchPool.Reset()
}

func (m *Matcher) Trace(value any) bool {
	convertedValue := m.scratchPool.ValueConvert(value)
	result := bool(C.mongory_matcher_trace(m.CPoint, convertedValue.CPoint))
	m.scratchPool.Reset()
	return result
}

func (m *Matcher) EnableTrace() {
	m.traceEnabled = true
	C.mongory_matcher_enable_trace(m.CPoint, m.scratchPool.CPoint)
}

func (m *Matcher) DisableTrace() {
	C.mongory_matcher_disable_trace(m.CPoint)
	m.scratchPool.Reset()
	m.traceEnabled = false
}

func (m *Matcher) PrintTrace() {
	if m.traceEnabled {
		C.mongory_matcher_print_trace(m.CPoint)
	}
}

func (m *Matcher) GetCondition() *map[string]any {
	return m.condition
}

func (m *Matcher) GetContext() *any {
	return m.context
}
