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

type Table struct {
	CPoint *C.mongory_table
	pool   *MemoryPool
}

func NewTable(pool *MemoryPool) *Table {
	table := C.mongory_table_new(pool.CPoint)
	return &Table{CPoint: table, pool: pool}
}

func (t *Table) Set(key string, value *Value) bool {
	result := C.go_mongory_table_set(t.CPoint, C.CString(key), value.CPoint)
	return bool(result)
}

func (t *Table) Get(key string) *Value {
	value := C.go_mongory_table_get(t.CPoint, C.CString(key))
	if value == nil {
		return nil
	}
	return &Value{CPoint: value, pool: t.pool}
}

func (t *Table) Delete(key string) bool {
	result := C.go_mongory_table_delete(t.CPoint, C.CString(key))
	return bool(result)
}
