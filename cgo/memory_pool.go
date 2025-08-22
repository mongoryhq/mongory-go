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
	"reflect"
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
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return NewValueUnsupported(m, value)
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		array := NewArray(m)
		for i := 0; i < rv.Len(); i++ {
			array.Push(m.ConditionConvert(rv.Index(i).Interface()))
		}
		return NewValueArray(m, array)
	case reflect.Map:
		table := NewTable(m)
		iter := rv.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			table.Set(key, m.ConditionConvert(iter.Value().Interface()))
		}
		return NewValueTable(m, table)
	case reflect.Ptr:
		return m.ConditionConvert(rv.Elem().Interface())
	default:
		return m.primitiveConvert(value)
	}
}

func (m *MemoryPool) ValueConvert(value any) *Value {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return NewValueUnsupported(m, value)
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		return NewValueShallowArray(m, NewShallowArray(m, value))
	case reflect.Map:
		return NewValueShallowTable(m, NewShallowTable(m, value))
	case reflect.Ptr:
		return m.ValueConvert(rv.Elem().Interface())
	default:
		return m.primitiveConvert(value)
	}
}

func (m *MemoryPool) primitiveConvert(value any) *Value {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return NewValueUnsupported(m, value)
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return NewValueInt(m, rv.Int())
	case reflect.Float32, reflect.Float64:
		return NewValueDouble(m, rv.Float())
	case reflect.String:
		return NewValueString(m, rv.String())
	case reflect.Bool:
		return NewValueBool(m, rv.Bool())
	default:
		return NewValueUnsupported(m, value)
	}
}
