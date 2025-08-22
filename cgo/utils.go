package cgo

/*
#include <stdbool.h>
#include <mongory-core.h>
#include <stdlib.h>
#include <stdint.h>

void *go_handle_to_ptr(uintptr_t h) { return (void *)h; }
uintptr_t go_ptr_to_handle(void *p) { return (uintptr_t)p; }
*/
import "C"
import (
	rcgo "runtime/cgo"
	"unsafe"
)

func handleToPtr(h rcgo.Handle) unsafe.Pointer {
	return C.go_handle_to_ptr(C.uintptr_t(uintptr(h)))
}

func ptrToHandle(ptr unsafe.Pointer) rcgo.Handle {
	return rcgo.Handle(C.go_ptr_to_handle(ptr))
}
