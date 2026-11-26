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
			opt.apply(&cfg)
		}
	}
	if err := cfg.validateError(); err != nil {
		return nil, err
	}
	doc, err := convertRootCondition(condition, cfg)
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
	if t == nil || t.root == nil {
		return false
	}
	return matchRaw(t.root, record)
}

// ExplainTree returns the compiled matcher tree as a structured ExplainNode.
// Returns nil if t is nil.
func (t *Tree) ExplainTree() *ExplainNode {
	if t == nil || t.root == nil {
		return nil
	}
	node := t.root.explainNode()
	return &node
}

// Explain returns a human-readable representation of the compiled matcher tree.
func (t *Tree) Explain() string {
	node := t.ExplainTree()
	if node == nil {
		return ""
	}
	var buf strings.Builder
	renderExplainNode(&buf, node, "", true, true)
	return buf.String()
}

// TraceTree evaluates record against the compiled condition and returns both
// the result and a structured per-node TraceNode tree.
func (t *Tree) TraceTree(record any) (bool, *TraceNode) {
	if t == nil || t.root == nil {
		return false, nil
	}
	matched, node := t.root.traceNode(record)
	return matched, &node
}

// Trace evaluates record against the compiled condition and returns both the
// result and a per-node execution trace.
func (t *Tree) Trace(record any) (bool, string) {
	matched, node := t.TraceTree(record)
	if node == nil {
		return false, ""
	}
	var buf strings.Builder
	renderTraceNode(&buf, node, "", true, true)
	return matched, buf.String()
}
