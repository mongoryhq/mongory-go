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
	"reflect"
	rcgo "runtime/cgo"
)

// ----- Go side: Shallow Array -----

type ShallowArray struct {
	CPoint *C.mongory_array
	target any
	pool   *MemoryPool
}

func NewShallowArray(pool *MemoryPool, values any) *ShallowArray {
	h := rcgo.NewHandle(values)
	pool.trackHandle(h)
	arr := &ShallowArray{
		CPoint: C.mongory_shallow_array_new(pool.CPoint, handleToPtr(h)),
		target: values,
		pool:   pool,
	}
	rv := reflect.ValueOf(values)
	var count int
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		count = rv.Len()
	}
	C.mongory_shallow_array_set_count(arr.CPoint, C.size_t(count))
	return arr
}

func (a *ShallowArray) Get(index int) *Value {
	rv := reflect.ValueOf(a.target)
	if !rv.IsValid() || rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return a.pool.ValueConvert(nil)
	}
	return a.pool.ValueConvert(rv.Index(index).Interface())
}

//export go_shallow_array_get
func go_shallow_array_get(a *C.go_mongory_array, index C.size_t) *C.mongory_value {
	pool := MemoryPool{CPoint: a.base.pool}
	h := ptrToHandle(a.go_array)
	target := h.Value()
	rv := reflect.ValueOf(target)
	var iv any
	if rv.IsValid() && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		iv = rv.Index(int(index)).Interface()
	}
	return pool.ValueConvert(iv).CPoint
}

//export go_shallow_array_to_string
func go_shallow_array_to_string(a *C.go_mongory_array) *C.char {
	h := ptrToHandle(a.go_array)
	target := h.Value()
	return C.CString(fmt.Sprintf("%v", target)) // TODO: implement
}

// ----- Go side: Shallow Table -----

type ShallowTable struct {
	CPoint *C.mongory_table
	target any
	pool   *MemoryPool
}

func NewShallowTable(pool *MemoryPool, values any) *ShallowTable {
	h := rcgo.NewHandle(values)
	pool.trackHandle(h)
	t := &ShallowTable{
		CPoint: C.mongory_shallow_table_new(pool.CPoint, handleToPtr(h)),
		target: values,
		pool:   pool,
	}
	// 設定項目數量（僅支援 map）
	rv := reflect.ValueOf(values)
	var count int
	if rv.IsValid() && rv.Kind() == reflect.Map {
		count = rv.Len()
	}
	C.mongory_shallow_table_set_count(t.CPoint, C.size_t(count))
	return t
}

func (t *ShallowTable) Get(key string) *Value {
	rv := reflect.ValueOf(t.target)
	if !rv.IsValid() || rv.Kind() != reflect.Map {
		return t.pool.ValueConvert(nil)
	}
	v := rv.MapIndex(reflect.ValueOf(key))
	if !v.IsValid() {
		return t.pool.ValueConvert(nil)
	}
	return t.pool.ValueConvert(v.Interface())
}

//export go_shallow_table_get
func go_shallow_table_get(a *C.go_mongory_table, key *C.char) *C.mongory_value {
	pool := MemoryPool{CPoint: a.base.pool}
	h := ptrToHandle(a.go_table)
	target := h.Value()
	rv := reflect.ValueOf(target)
	var iv any
	if rv.IsValid() && rv.Kind() == reflect.Map {
		v := rv.MapIndex(reflect.ValueOf(C.GoString(key)))
		if v.IsValid() {
			iv = v.Interface()
		}
	}
	return pool.ValueConvert(iv).CPoint
}

//export go_shallow_table_to_string
func go_shallow_table_to_string(t *C.go_mongory_table) *C.char {
	h := ptrToHandle(t.go_table)
	target := h.Value()
	return C.CString(fmt.Sprintf("%v", target)) // TODO: implement
}
