package cgo

/*
#cgo CFLAGS: -I${SRCDIR}/binding/include -I${SRCDIR}/binding/src -I${SRCDIR}/../mongory-core/include
#cgo LDFLAGS: -lm
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>

*/
import "C"

func Init() {
	C.mongory_init()
}

func Cleanup() {
	C.mongory_cleanup()
}
