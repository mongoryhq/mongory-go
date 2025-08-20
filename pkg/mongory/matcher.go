package mongory

import "mongory/cgo"

type Matcher interface {
	Match(value any) bool
	Explain()
	Trace(value any) bool
	PrintTrace()
	EnableTrace()
	DisableTrace()
	GetCondition() *map[string]any
	GetContext() *any
}

func NewMatcher(condition map[string]any, context *any) Matcher {
	pool := cgo.NewMemoryPool()
	return cgo.NewMatcher(pool, condition, context)
}
