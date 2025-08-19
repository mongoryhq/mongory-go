package mongory

/*
#include "mongory_wrapper.h"
*/
import "C"
import (
	"fmt"
)

func Ping() error {
	C.mongory_init()
	pool := C.mongory_memory_pool_new()
	i := C.mongory_value_wrap_i(pool, C.int(42))
	t := C.mongory_type_to_string(i)
	goStr := C.GoString(t)
	fmt.Println("goStr", goStr)
	table := C.mongory_table_new(pool)
	C.go_mongory_table_set(table, C.CString("key"), i)
	i2 := C.go_mongory_table_get(table, C.CString("key"))
	t2 := C.mongory_type_to_string(i2)
	goStr2 := C.GoString(t2)
	fmt.Println("goStr2", goStr2)
	C.go_mongory_table_delete(table, C.CString("key"))
	i3 := C.go_mongory_table_get(table, C.CString("key"))
	if i3 != nil {
		t3 := C.mongory_type_to_string(i3)
		goStr3 := C.GoString(t3)
		fmt.Println("goStr3", goStr3)
	} else {
		fmt.Println("i3 is nil")
	}

	C.go_mongory_memory_pool_free(pool)
	C.mongory_cleanup()
	return nil
}
