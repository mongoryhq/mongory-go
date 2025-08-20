package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>

bool go_mongory_table_set(mongory_table* t, char* key, mongory_value* v) {
	return t->set(t, key, v);
}

mongory_value* go_mongory_table_get(mongory_table* t, char* key) {
	return t->get(t, key);
}

bool go_mongory_table_delete(mongory_table* t, char* key) {
	return t->del(t, key);
}
*/
import "C"
import "unsafe"

type Table struct {
	CPoint *C.mongory_table
	pool   *MemoryPool
}

func NewTable(pool *MemoryPool) *Table {
	table := C.mongory_table_new(pool.CPoint)
	return &Table{CPoint: table, pool: pool}
}

func (t *Table) Set(key string, value *Value) bool {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	result := C.go_mongory_table_set(t.CPoint, ckey, value.CPoint)
	return bool(result)
}

func (t *Table) Get(key string) *Value {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	value := C.go_mongory_table_get(t.CPoint, ckey)
	if value == nil {
		return nil
	}
	return &Value{CPoint: value, pool: t.pool}
}

func (t *Table) Delete(key string) bool {
	ckey := C.CString(key)
	defer C.free(unsafe.Pointer(ckey))
	result := C.go_mongory_table_delete(t.CPoint, ckey)
	return bool(result)
}
