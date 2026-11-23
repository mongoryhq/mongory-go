package engine

import (
	"errors"
	"regexp"
	"testing"
)

func TestNilAndZeroTreeFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		tree *Tree
	}{
		{name: "nil", tree: nil},
		{name: "zero", tree: &Tree{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.tree.Match(map[string]any{"a": 1}) {
				t.Fatalf("Match = true, want false")
			}
			if got := tc.tree.Explain(); got != "" {
				t.Fatalf("Explain = %q, want empty", got)
			}
			if matched, trace := tc.tree.Trace(map[string]any{"a": 1}); matched || trace != "" {
				t.Fatalf("Trace = (%v, %q), want (false, \"\")", matched, trace)
			}
		})
	}
}

func TestDefensiveNilRegexpBuilders(t *testing.T) {
	var nilRe *regexp.Regexp
	ctx := newCompileCtx(CompileConfig{MaxDepth: DefaultMaxDepth, MaxNodes: DefaultMaxNodes})

	if _, err := buildRegex(ctx, nilRe, ""); !errors.Is(err, ErrInvalidCondition) {
		t.Fatalf("buildRegex(nil *regexp.Regexp) = %v, want ErrInvalidCondition", err)
	}

	m, err := buildLiteral(ctx, nilRe)
	if err != nil {
		t.Fatalf("buildLiteral(nil *regexp.Regexp): %v", err)
	}
	if !m.match(nil) || m.match("abc") {
		t.Fatalf("buildLiteral(nil *regexp.Regexp) did not behave as nullMatcher")
	}

	arrM, err := buildArrayRecord(nilRe, ctx)
	if err != nil {
		t.Fatalf("buildArrayRecord(nil *regexp.Regexp): %v", err)
	}
	if !arrM.match([]any{nil}) || arrM.match([]any{"abc"}) {
		t.Fatalf("buildArrayRecord(nil *regexp.Regexp) did not match nil element safely")
	}
}
