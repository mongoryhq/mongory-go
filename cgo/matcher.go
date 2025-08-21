package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>

*/
import "C"
import (
	"errors"
	"unsafe"
)

type Matcher struct {
	CPoint       *C.mongory_matcher
	condition    *map[string]any
	context      *any
	pool         *MemoryPool
	scratchPool  *MemoryPool
	tracePool    *MemoryPool
	traceEnabled bool
}

func NewMatcher(pool *MemoryPool, condition map[string]any, context *any) (*Matcher, error) {
	conditionValue := pool.ConditionConvert(condition)
	if conditionValue == nil {
		return nil, errors.New(pool.GetError())
	}
	scratchPool := NewMemoryPool()
	cpoint := C.mongory_matcher_new(pool.CPoint, conditionValue.CPoint, unsafe.Pointer(context))
	if cpoint == nil {
		return nil, errors.New(pool.GetError())
	}
	return &Matcher{
		CPoint:       cpoint,
		condition:    &condition,
		context:      context,
		pool:         pool,
		scratchPool:  scratchPool,
		tracePool:    nil,
		traceEnabled: false,
	}, nil
}

func (m *Matcher) Match(value any) (bool, error) {
	convertedValue := m.scratchPool.ValueConvert(value)
	if convertedValue == nil {
		return false, errors.New(m.scratchPool.GetError())
	}
	result := bool(C.mongory_matcher_match(m.CPoint, convertedValue.CPoint))
	m.scratchPool.Reset()

	return result, nil
}

func (m *Matcher) Explain() error {
	C.mongory_matcher_explain(m.CPoint, m.scratchPool.CPoint)
	if m.scratchPool.GetError() != "" {
		return errors.New(m.scratchPool.GetError())
	}
	m.scratchPool.Reset()
	return nil
}

func (m *Matcher) Trace(value any) (bool, error) {
	tracePool := NewMemoryPool()
	convertedValue := tracePool.ValueConvert(value)
	if convertedValue == nil {
		return false, errors.New(tracePool.GetError())
	}
	result := bool(C.mongory_matcher_trace(m.CPoint, convertedValue.CPoint))
	tracePool.Free()
	return result, nil
}

func (m *Matcher) EnableTrace() error {
	m.traceEnabled = true
	if m.tracePool == nil {
		m.tracePool = NewMemoryPool()
	}
	C.mongory_matcher_enable_trace(m.CPoint, m.tracePool.CPoint)
	if m.tracePool.GetError() != "" {
		return errors.New(m.tracePool.GetError())
	}
	return nil
}

func (m *Matcher) DisableTrace() error {
	if !m.traceEnabled {
		return nil
	}
	C.mongory_matcher_disable_trace(m.CPoint)
	m.tracePool.Free()
	m.tracePool = nil
	m.traceEnabled = false
	return nil
}

func (m *Matcher) PrintTrace() error {
	if !m.traceEnabled {
		return nil
	}
	C.mongory_matcher_print_trace(m.CPoint)
	if m.tracePool.GetError() != "" {
		return errors.New(m.tracePool.GetError())
	}

	return nil
}

func (m *Matcher) GetCondition() *map[string]any {
	return m.condition
}

func (m *Matcher) GetContext() *any {
	return m.context
}

func (m *Matcher) Free() {
	m.scratchPool.Free()
	m.pool.Free()
	if m.tracePool != nil {
		m.tracePool.Free()
	}
}
