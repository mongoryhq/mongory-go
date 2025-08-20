package cgo

/*
#cgo CFLAGS: -I${SRCDIR}/../mongory-core/include
#cgo LDFLAGS: ${SRCDIR}/../mongory-core/mongory-core.a -lm
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
