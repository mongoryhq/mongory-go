package mongory

import (
	"runtime"

	"github.com/mongoryhq/mongory-go/cgo"
)

type CMatcher interface {
	Match(value any) (bool, error)
	Explain() error
	Trace(value any) (bool, error)
	PrintTrace() error
	EnableTrace() error
	DisableTrace() error
	GetCondition() *map[string]any
	GetContext() *any
}

func NewCMatcher(condition map[string]any, context *any) (CMatcher, error) {
	matcher, err := cgo.NewMatcher(condition, context)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(matcher, func(m *cgo.Matcher) {
		m.Free()
	})
	return matcher, nil
}
