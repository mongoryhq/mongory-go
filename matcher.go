package mongory

import (
	"runtime"

	"github.com/mongoryhq/mongory-go/cgo"
)

type Matcher interface {
	Match(value any) (bool, error)
	Explain() error
	Trace(value any) (bool, error)
	PrintTrace() error
	EnableTrace() error
	DisableTrace() error
	GetCondition() *map[string]any
	GetContext() *any
}

func NewMatcher(condition map[string]any, context *any) (Matcher, error) {
	pool := cgo.NewMemoryPool()
	matcher, err := cgo.NewMatcher(pool, condition, context)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(matcher, func(m *cgo.Matcher) {
		m.Free()
	})
	return matcher, nil
}
