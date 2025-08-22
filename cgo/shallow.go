package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>
#include <stdint.h>

// ----- Array bridge -----

typedef struct go_mongory_array {
	mongory_array base;
	void *go_array;
} go_mongory_array;

extern mongory_value *go_shallow_array_get(go_mongory_array *a, size_t index);

static mongory_value *cgo_shallow_array_get(mongory_array *a, size_t index) {
	return go_shallow_array_get((go_mongory_array *)a, index);
}

static mongory_array *mongory_shallow_array_new(mongory_memory_pool *pool, void *go_array) {
	go_mongory_array *a = pool->alloc(pool, sizeof(go_mongory_array));
	a->base.get = cgo_shallow_array_get;
	a->base.pool = pool;
	a->go_array = go_array;
	return &a->base;
}

static void mongory_shallow_array_set_count(mongory_array *a, size_t count) {
	a->count = count;
}

// ----- Table bridge -----

typedef struct go_mongory_table {
	mongory_table base;
	void *go_table;
} go_mongory_table;

extern mongory_value *go_shallow_table_get(go_mongory_table *a, char *key);

static mongory_value *cgo_shallow_table_get(mongory_table *a, char *key) {
	return go_shallow_table_get((go_mongory_table *)a, key);
}

static mongory_table *mongory_shallow_table_new(mongory_memory_pool *pool, void *go_table) {
	go_mongory_table *a = pool->alloc(pool, sizeof(go_mongory_table));
	a->base.pool = pool;
	a->go_table = go_table;
	a->base.get = cgo_shallow_table_get;
	return &a->base;
}

static void mongory_shallow_table_set_count(mongory_table *a, size_t count) {
	a->count = count;
}

*/
import "C"
import (
	"fmt"
	rcgo "runtime/cgo"
)

// ----- Go side: Shallow Array -----

type ShallowArray struct {
	CPoint *C.mongory_array
	target []any
	pool   *MemoryPool
}

func NewShallowArray(pool *MemoryPool, values []any) *ShallowArray {
	h := rcgo.NewHandle(values)
	pool.trackHandle(h)
	arr := &ShallowArray{
		CPoint: C.mongory_shallow_array_new(pool.CPoint, handleToPtr(h)),
		target: values,
		pool:   pool,
	}
	C.mongory_shallow_array_set_count(arr.CPoint, C.size_t(len(values)))
	return arr
}

func (a *ShallowArray) Get(index int) *Value {
	return a.pool.ValueConvert(a.target[index])
}

//export go_shallow_array_get
func go_shallow_array_get(a *C.go_mongory_array, index C.size_t) *C.mongory_value {
	pool := MemoryPool{CPoint: a.base.pool}
	h := ptrToHandle(a.go_array)
	target := h.Value().([]any)
	return pool.ValueConvert(target[int(index)]).CPoint
}

//export go_shallow_array_to_string
func go_shallow_array_to_string(a *C.go_mongory_array) *C.char {
	h := ptrToHandle(a.go_array)
	target := h.Value().([]any)
	return C.CString(fmt.Sprintf("%v", target)) // TODO: implement
}

// ----- Go side: Shallow Table -----

type ShallowTable struct {
	CPoint *C.mongory_table
	target map[string]any
	pool   *MemoryPool
}

func NewShallowTable(pool *MemoryPool, values map[string]any) *ShallowTable {
	h := rcgo.NewHandle(values)
	pool.trackHandle(h)
	t := &ShallowTable{
		CPoint: C.mongory_shallow_table_new(pool.CPoint, handleToPtr(h)),
		target: values,
		pool:   pool,
	}
	C.mongory_shallow_table_set_count(t.CPoint, C.size_t(len(values)))
	return t
}

func (t *ShallowTable) Get(key string) *Value {
	return t.pool.ValueConvert(t.target[key])
}

//export go_shallow_table_get
func go_shallow_table_get(a *C.go_mongory_table, key *C.char) *C.mongory_value {
	pool := MemoryPool{CPoint: a.base.pool}
	h := ptrToHandle(a.go_table)
	target := h.Value().(map[string]any)
	return pool.ValueConvert(target[C.GoString(key)]).CPoint
}

//export go_shallow_table_to_string
func go_shallow_table_to_string(t *C.go_mongory_table) *C.char {
	h := ptrToHandle(t.go_table)
	target := h.Value().(map[string]any)
	return C.CString(fmt.Sprintf("%v", target)) // TODO: implement
}
