// Package engine is the pure Go matcher tree behind package mongory.
// Only Compile, Tree and ErrInvalidCondition are meant for the facade.
package engine

import "errors"

// ErrInvalidCondition is wrapped by every error returned from Compile for a
// malformed condition.
var ErrInvalidCondition = errors.New("mongory: invalid condition")

// Tree is a compiled condition. It is safe for concurrent use.
type Tree struct {
	root matcher
}

// Compile builds the matcher tree once; Match then only walks it. condition
// is any document: map[string]any, bson.M, bson.D, a struct, or nil.
func Compile(condition any) (*Tree, error) {
	var doc map[string]any
	switch c := normalize(condition).(type) {
	case nil:
	case map[string]any:
		doc = c
	default:
		return nil, invalidCondition("condition must be a document, got %T", condition)
	}
	root, err := buildHashCondition(convertCondition(doc))
	if err != nil {
		return nil, err
	}
	return &Tree{root: root}, nil
}

// Match reports whether record satisfies the compiled condition.
func (t *Tree) Match(record any) bool {
	return t.root.match(normalize(record))
}
