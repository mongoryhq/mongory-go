package mongory

import "mongory/cgo"

func Init() {
	cgo.Init()
}

func Cleanup() {
	cgo.Cleanup()
}
