package mongory

import (
	"sync"

	"github.com/mongoryhq/mongory-go/cgo"
)

func Cleanup() {
	cgo.Cleanup()
}

var once sync.Once

func Init() {
	once.Do(func() {
		cgo.Init()
	})
}

func init() {
	Init()
}
