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

func (m *MemoryPool) ValueConvert(value any) *Value {
	switch value.(type) {
	case int:
		return NewValueInt(m, int64(value.(int)))
	case int8:
		return NewValueInt(m, int64(value.(int8)))
	case int16:
		return NewValueInt(m, int64(value.(int16)))
	case int32:
		return NewValueInt(m, int64(value.(int32)))
	case int64:
		return NewValueInt(m, value.(int64))
	case float32:
		return NewValueDouble(m, float64(value.(float32)))
	case float64:
		return NewValueDouble(m, value.(float64))
	case string:
		return NewValueString(m, value.(string))
	case bool:
		return NewValueBool(m, value.(bool))
	case []any:
		array := NewArray(m)
		for _, v := range value.([]any) {
			array.Push(m.ValueConvert(v))
		}
		return NewValueArray(m, array)
	case map[string]any:
		table := NewTable(m)
		for k, v := range value.(map[string]any) {
			table.Set(k, m.ValueConvert(v))
		}
		return NewValueTable(m, table)
	case nil:
		return NewValueNull(m)
	case *Value:
		return value.(*Value)
	case *Array:
		return NewValueArray(m, value.(*Array))
	case *Table:
		return NewValueTable(m, value.(*Table))
	default:
		return NewValueUnsupported(m, value)
	}
}
