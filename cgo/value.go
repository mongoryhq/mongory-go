package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>
#include <stdint.h>

void* go_handle_to_ptr(uintptr_t h) { return (void*)h; }
uintptr_t go_ptr_to_handle(void* p) { return (uintptr_t)p; }

char * go_mongory_value_to_string(mongory_value* v, mongory_memory_pool* pool) {
	return v->to_str(v, pool);
}

bool go_mongory_value_get_bool(mongory_value* v) {
  return v->data.b;
}

int go_mongory_value_get_int(mongory_value* v) {
  return v->data.i;
}

char * go_mongory_value_get_string(mongory_value* v) {
  return v->data.s;
}

double go_mongory_value_get_double(mongory_value* v) {
  return v->data.d;
}

mongory_array * go_mongory_value_get_array(mongory_value* v) {
  return v->data.a;
}

mongory_table * go_mongory_value_get_table(mongory_value* v) {
  return v->data.t;
}

void * go_mongory_value_get_ptr(mongory_value* v) {
  return v->data.ptr;
}

void * go_mongory_value_get_u(mongory_value* v) {
  return v->data.u;
}

void * go_mongory_value_get_regex(mongory_value* v) {
  return v->data.regex;
}

void * go_mongory_value_get_null(mongory_value* v) {
  return v->data.u;
}

// Forward declarations for shallow to_string
extern char *go_shallow_array_to_string(void *go_array, mongory_memory_pool *pool);
extern char *go_shallow_table_to_string(void *go_table, mongory_memory_pool *pool);

static char *cgo_shallow_array_to_string(mongory_value *v, mongory_memory_pool *pool) {
	return go_shallow_array_to_string(v->data.a, pool);
}

static char *cgo_shallow_table_to_string(mongory_value *v, mongory_memory_pool *pool) {
	return go_shallow_table_to_string(v->data.t, pool);
}

static void mongory_value_set_array_to_string(mongory_value *v) {
	v->to_str = cgo_shallow_array_to_string;
}

static void mongory_value_set_table_to_string(mongory_value *v) {
	v->to_str = cgo_shallow_table_to_string;
}

*/
import "C"
import (
	rcgo "runtime/cgo"
	"unsafe"
)

type MongoryType int

const (
	MONGORY_TYPE_NULL        MongoryType = C.MONGORY_TYPE_NULL
	MONGORY_TYPE_BOOL        MongoryType = C.MONGORY_TYPE_BOOL
	MONGORY_TYPE_INT         MongoryType = C.MONGORY_TYPE_INT
	MONGORY_TYPE_DOUBLE      MongoryType = C.MONGORY_TYPE_DOUBLE
	MONGORY_TYPE_STRING      MongoryType = C.MONGORY_TYPE_STRING
	MONGORY_TYPE_ARRAY       MongoryType = C.MONGORY_TYPE_ARRAY
	MONGORY_TYPE_TABLE       MongoryType = C.MONGORY_TYPE_TABLE
	MONGORY_TYPE_REGEX       MongoryType = C.MONGORY_TYPE_REGEX
	MONGORY_TYPE_POINTER     MongoryType = C.MONGORY_TYPE_POINTER
	MONGORY_TYPE_UNSUPPORTED MongoryType = C.MONGORY_TYPE_UNSUPPORTED
)

type Value struct {
	CPoint *C.mongory_value
	Type   MongoryType
	pool   *MemoryPool
}

func NewValueInt(pool *MemoryPool, i int64) *Value { // as integer
	return &Value{CPoint: C.mongory_value_wrap_i(pool.CPoint, C.int64_t(i)), Type: MONGORY_TYPE_INT, pool: pool}
}

func NewValueString(pool *MemoryPool, s string) *Value { // as string
	cs := C.CString(s)
	v := &Value{CPoint: C.mongory_value_wrap_s(pool.CPoint, cs), Type: MONGORY_TYPE_STRING, pool: pool}
	C.free(unsafe.Pointer(cs))
	return v
}

func NewValueBool(pool *MemoryPool, b bool) *Value { // as boolean
	return &Value{CPoint: C.mongory_value_wrap_b(pool.CPoint, C.bool(b)), Type: MONGORY_TYPE_BOOL, pool: pool}
}

func NewValueDouble(pool *MemoryPool, d float64) *Value { // as double
	return &Value{CPoint: C.mongory_value_wrap_d(pool.CPoint, C.double(d)), Type: MONGORY_TYPE_DOUBLE, pool: pool}
}

func NewValueArray(pool *MemoryPool, a *Array) *Value { // as array
	return &Value{CPoint: C.mongory_value_wrap_a(pool.CPoint, a.CPoint), Type: MONGORY_TYPE_ARRAY, pool: pool}
}

func NewValueShallowArray(pool *MemoryPool, a *ShallowArray) *Value { // as array
	value := &Value{CPoint: C.mongory_value_wrap_a(pool.CPoint, a.CPoint), Type: MONGORY_TYPE_ARRAY, pool: pool}
	C.mongory_value_set_array_to_string(value.CPoint)
	return value
}

func NewValueTable(pool *MemoryPool, t *Table) *Value { // as table
	return &Value{CPoint: C.mongory_value_wrap_t(pool.CPoint, t.CPoint), Type: MONGORY_TYPE_TABLE, pool: pool}
}

func NewValueShallowTable(pool *MemoryPool, t *ShallowTable) *Value { // as table
	value := &Value{CPoint: C.mongory_value_wrap_t(pool.CPoint, t.CPoint), Type: MONGORY_TYPE_TABLE, pool: pool}
	C.mongory_value_set_table_to_string(value.CPoint)
	return value
}

func NewValueRegex(pool *MemoryPool, regex any) *Value { // as regex (store Go handle)
	h := rcgo.NewHandle(regex)
	pool.trackHandle(h)
	ptr := C.go_handle_to_ptr(C.uintptr_t(uintptr(h)))
	return &Value{CPoint: C.mongory_value_wrap_regex(pool.CPoint, ptr), Type: MONGORY_TYPE_REGEX, pool: pool}
}

func NewValuePointer(pool *MemoryPool, ptr any) *Value { // as generic pointer (store Go handle)
	h := rcgo.NewHandle(ptr)
	pool.trackHandle(h)
	vptr := C.go_handle_to_ptr(C.uintptr_t(uintptr(h)))
	return &Value{CPoint: C.mongory_value_wrap_ptr(pool.CPoint, vptr), Type: MONGORY_TYPE_POINTER, pool: pool}
}

func NewValueUnsupported(pool *MemoryPool, u any) *Value { // as unsupported type (store Go handle)
	h := rcgo.NewHandle(u)
	pool.trackHandle(h)
	uptr := C.go_handle_to_ptr(C.uintptr_t(uintptr(h)))
	return &Value{CPoint: C.mongory_value_wrap_u(pool.CPoint, uptr), Type: MONGORY_TYPE_UNSUPPORTED, pool: pool}
}

func NewValueNull(pool *MemoryPool) *Value { // as null pointer
	return &Value{CPoint: C.mongory_value_wrap_n(pool.CPoint, nil), Type: MONGORY_TYPE_NULL, pool: pool}
}

func (v *Value) ToString() string {
	stringValue := C.go_mongory_value_to_string(v.CPoint, v.pool.CPoint)
	if stringValue == nil {
		return ""
	}
	return C.GoString(stringValue)
}

func (v *Value) GetType() string {
	switch v.Type {
	case MONGORY_TYPE_NULL:
		return "null"
	case MONGORY_TYPE_BOOL:
		return "bool"
	case MONGORY_TYPE_INT:
		return "int"
	case MONGORY_TYPE_DOUBLE:
		return "double"
	case MONGORY_TYPE_STRING:
		return "string"
	case MONGORY_TYPE_ARRAY:
		return "array"
	case MONGORY_TYPE_TABLE:
		return "table"
	case MONGORY_TYPE_REGEX:
		return "regex"
	case MONGORY_TYPE_POINTER:
		return "pointer"
	case MONGORY_TYPE_UNSUPPORTED:
		return "unsupported"
	}
	return ""
}

func (v *Value) GetInt() int64 {
	intValue := C.go_mongory_value_get_int(v.CPoint)
	if intValue == 0 {
		return 0
	}
	return int64(intValue)
}

func (v *Value) GetString() string {
	stringValue := C.go_mongory_value_get_string(v.CPoint)
	if stringValue == nil {
		return ""
	}
	return C.GoString(stringValue)
}

func (v *Value) GetBool() bool {
	boolValue := C.go_mongory_value_get_bool(v.CPoint)
	if !boolValue {
		return false
	}
	return bool(boolValue)
}

func (v *Value) GetDouble() float64 {
	doubleValue := C.go_mongory_value_get_double(v.CPoint)
	if doubleValue == 0.0 {
		return 0.0
	}
	return float64(doubleValue)
}

func (v *Value) GetArray() *Array {
	array := C.go_mongory_value_get_array(v.CPoint)
	if array == nil {
		return nil
	}
	return &Array{CPoint: array, pool: v.pool}
}

func (v *Value) GetTable() *Table {
	table := C.go_mongory_value_get_table(v.CPoint)
	if table == nil {
		return nil
	}
	return &Table{CPoint: table, pool: v.pool}
}

func (v *Value) GetPointer() unsafe.Pointer {
	ptr := C.go_mongory_value_get_ptr(v.CPoint)
	if ptr == nil {
		return nil
	}
	return unsafe.Pointer(ptr)
}

func (v *Value) GetUnsupported() any {
	ptr := C.go_mongory_value_get_u(v.CPoint)
	if ptr == nil {
		return nil
	}
	h := rcgo.Handle(C.go_ptr_to_handle(ptr))
	return h.Value()
}

func (v *Value) GetNull() any {
	return nil
}

func (v *Value) GetRegexHandle() rcgo.Handle {
	ptr := C.go_mongory_value_get_regex(v.CPoint)
	if ptr == nil {
		return rcgo.Handle(0)
	}
	return rcgo.Handle(C.go_ptr_to_handle(ptr))
}

func (v *Value) GetPointerHandle() rcgo.Handle {
	ptr := C.go_mongory_value_get_ptr(v.CPoint)
	if ptr == nil {
		return rcgo.Handle(0)
	}
	return rcgo.Handle(C.go_ptr_to_handle(ptr))
}
