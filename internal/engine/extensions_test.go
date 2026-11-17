package engine

import "testing"

func TestBuildArrayRecordInheritsConfigWithoutLimits(t *testing.T) {
	cfg := CompileConfig{
		MaxDepth: 4,
		MaxNodes: 2,
	}
	WithOptions(
		WithTypeConverter(func(id fakeOID) any { return string(id[:]) }),
		WithOperator("$rawHas", Operator{
			MatchElements: true,
			Compile: func(arg any) (Predicate, error) {
				want := arg.(string)
				return func(v any, exists bool) bool {
					s, ok := v.(string)
					return exists && ok && s == want
				}, nil
			},
		}),
	).apply(&cfg)

	ctx := newCompileCtx(cfg)
	// Simulate many element checks exceeding the small MaxNodes limit of 2 if limits applied.
	cond := map[string]any{
		"$rawHas": "go",
		"$in":     []any{"go", "rust"},
		"$ne":     "bad",
	}
	lit, err := buildLiteral(newCompileCtx(CompileConfig{
		converters: cfg.converters,
		operators:  cfg.operators,
	}), cond)
	if err != nil {
		t.Fatalf("buildLiteral failed: %v", err)
	}
	extLit, ok := lit.(*extLiteralMatcher)
	if !ok || extLit.arrayCtx == nil {
		t.Fatal("expected *extLiteralMatcher with non-nil arrayCtx when extensions configured")
	}
	if extLit.arrayCtx.maxDepth != 0 || extLit.arrayCtx.maxNodes != 0 {
		t.Fatalf("expected zeroed limits on arrayCtx, got depth=%d nodes=%d", extLit.arrayCtx.maxDepth, extLit.arrayCtx.maxNodes)
	}
	m, err := buildArrayRecord(extLit.condition, extLit.arrayCtx)
	if err != nil {
		t.Fatalf("buildArrayRecord failed: %v", err)
	}
	if !m.match([]any{"go"}) || m.match([]any{"rust"}) {
		t.Fatalf("arrayRecord inherited custom op mismatch: go=%v rust=%v", m.match([]any{"go"}), m.match([]any{"rust"}))
	}
	_ = ctx
}

type fakeOID [4]byte

func TestMergeExpandedNodeLeafCollisions(t *testing.T) {
	doc := map[string]any{"a": map[string]any{"b": 2}}
	res := mergeExpandedNode(doc, map[string]any{"a": map[string]any{"b": 1, "c": 3}})
	if doc["a"].(map[string]any)["c"] != 3 {
		t.Fatalf("expected non-conflicting key c to merge, got %#v", doc)
	}
	if doc["a"].(map[string]any)["b"] != 2 {
		t.Fatalf("expected existing leaf b=2 preserved, got %#v", doc)
	}
	if res == nil || res["a"].(map[string]any)["b"] != 1 {
		t.Fatalf("expected residual conflict a.b=1, got %#v", res)
	}
}
