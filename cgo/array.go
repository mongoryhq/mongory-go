package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>

mongory_value *go_mongory_array_get(mongory_array* a, size_t index) {
	return a->get(a, index);
}

bool go_mongory_array_set(mongory_array* a, size_t index, mongory_value* v) {
	return a->set(a, index, v);
}

bool go_mongory_array_push(mongory_array* a, mongory_value* v) {
	return a->push(a, v);
}

*/
import "C"

type Array struct {
	CPoint *C.mongory_array
	pool   *MemoryPool
}

func NewArray(pool *MemoryPool) *Array {
	return &Array{CPoint: C.mongory_array_new(pool.CPoint), pool: pool}
}

func (a *Array) Get(index int) *Value {
	value := C.go_mongory_array_get(a.CPoint, C.size_t(index))
	if value == nil {
		return nil
	}
	return &Value{CPoint: value, pool: a.pool}
}

func (a *Array) Set(index int, value *Value) bool {
	return bool(C.go_mongory_array_set(a.CPoint, C.size_t(index), value.CPoint))
}

func (a *Array) Push(value *Value) bool {
	return bool(C.go_mongory_array_push(a.CPoint, value.CPoint))
}

func (a *Array) Len() int {
	return int(a.CPoint.count)
}
