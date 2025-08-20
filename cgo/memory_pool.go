package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>

void *go_mongory_memory_pool_alloc(mongory_memory_pool* pool, size_t size) {
	return pool->alloc(pool, size);
}

void go_mongory_memory_pool_reset(mongory_memory_pool* pool) {
	pool->reset(pool);
}

void go_mongory_memory_pool_free(mongory_memory_pool* pool) {
	pool->free(pool);
}
*/
import "C"
import (
	rcgo "runtime/cgo"
)

type MemoryPool struct {
	CPoint  *C.mongory_memory_pool
	handles []rcgo.Handle
}

func NewMemoryPool() *MemoryPool {
	pool := C.mongory_memory_pool_new()
	return &MemoryPool{CPoint: pool, handles: make([]rcgo.Handle, 0)}
}

func (m *MemoryPool) trackHandle(h rcgo.Handle) {
	m.handles = append(m.handles, h)
}

func (m *MemoryPool) Reset() {
	C.go_mongory_memory_pool_reset(m.CPoint)
	for _, h := range m.handles {
		h.Delete()
	}
	m.handles = m.handles[:0]
}

func (m *MemoryPool) Free() {
	C.go_mongory_memory_pool_free(m.CPoint)
	for _, h := range m.handles {
		h.Delete()
	}
	m.handles = nil
}

func (m *MemoryPool) GetError() string {
	err := m.CPoint.error
	if err == nil {
		return ""
	}
	return C.GoString(err.message)
}

func (m *MemoryPool) ConditionConvert(value any) *Value {
	switch value := value.(type) {

	case []any:
		array := NewArray(m)
		for _, v := range value {
			array.Push(m.ConditionConvert(v))
		}
		return NewValueArray(m, array)
	case map[string]any:
		table := NewTable(m)
		for k, v := range value {
			table.Set(k, m.ConditionConvert(v))
		}
		return NewValueTable(m, table)
	default:
		return m.primitiveConvert(value)
	}
}

func (m *MemoryPool) ValueConvert(value any) *Value {
	switch value := value.(type) {

	case []any:
		return NewValueShallowArray(m, NewShallowArray(m, value))
	case map[string]any:
		return NewValueShallowTable(m, NewShallowTable(m, value))
	default:
		return m.primitiveConvert(value)
	}
}

func (m *MemoryPool) primitiveConvert(value any) *Value {
	switch value := value.(type) {
	case *Value:
		return value
	case *Array:
		return NewValueArray(m, value)
	case *Table:
		return NewValueTable(m, value)
	case int:
		return NewValueInt(m, int64(value))
	case int8:
		return NewValueInt(m, int64(value))
	case int16:
		return NewValueInt(m, int64(value))
	case int32:
		return NewValueInt(m, int64(value))
	case int64:
		return NewValueInt(m, value)
	case float32:
		return NewValueDouble(m, float64(value))
	case float64:
		return NewValueDouble(m, value)
	case string:
		return NewValueString(m, value)
	case bool:
		return NewValueBool(m, value)
	case nil:
		return NewValueNull(m)
	default:
		return NewValueUnsupported(m, value)
	}
}
