// Package engine is the pure Go matcher tree behind package mongory.
// Only Compile, Tree and ErrInvalidCondition are meant for the facade.
package engine

import (
	"errors"
	"strings"
)

// ErrInvalidCondition is wrapped by every error returned from Compile for a
// malformed condition.
var ErrInvalidCondition = errors.New("mongory: invalid condition")

// ErrLimitExceeded is wrapped (alongside ErrInvalidCondition) when a condition
// exceeds the configured MaxDepth or MaxNodes limit.
var ErrLimitExceeded = errors.New("condition limit exceeded")

const (
	// DefaultMaxDepth caps condition nesting depth during compilation.
	DefaultMaxDepth = 64
	// DefaultMaxNodes caps the number of compiled matcher nodes.
	DefaultMaxNodes = 4096
)

// CompileConfig controls compile-time resource limits.
type CompileConfig struct {
	MaxDepth int
	MaxNodes int
}

// Option configures Compile behavior.
type Option func(*CompileConfig)

// WithMaxDepth overrides DefaultMaxDepth (64). A non-positive value disables
// the depth limit.
func WithMaxDepth(depth int) Option {
	return func(c *CompileConfig) { c.MaxDepth = depth }
}

// WithMaxNodes overrides DefaultMaxNodes (4096). A non-positive value disables
// the node limit.
func WithMaxNodes(nodes int) Option {
	return func(c *CompileConfig) { c.MaxNodes = nodes }
}

// Tree is a compiled condition. It is safe for concurrent use.
type Tree struct {
	root matcher
}

// Compile builds the matcher tree once; Match then only walks it. condition
// is any document: map[string]any, bson.M, bson.D, a struct, or nil.
func Compile(condition any, opts ...Option) (*Tree, error) {
	cfg := CompileConfig{
		MaxDepth: DefaultMaxDepth,
		MaxNodes: DefaultMaxNodes,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	doc, err := convertRootCondition(condition, cfg.MaxDepth)
	if err != nil {
		return nil, err
	}
	root, err := buildHashCondition(newCompileCtx(cfg), doc)
	if err != nil {
		return nil, err
	}
	return &Tree{root: root}, nil
}

// Match reports whether record satisfies the compiled condition.
func (t *Tree) Match(record any) bool {
	return matchRaw(t.root, record)
}

// Explain returns a human-readable representation of the compiled matcher tree.
func (t *Tree) Explain() string {
	var buf strings.Builder
	t.root.explain(&buf, "", true, true)
	return buf.String()
}

// Trace evaluates record against the compiled condition and returns both the
// result and a per-node execution trace.
func (t *Tree) Trace(record any) (bool, string) {
	var buf strings.Builder
	matched := t.root.trace(normalize(record), &buf, "", true, true)
	return matched, buf.String()
}
