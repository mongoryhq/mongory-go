package engine

import (
	"strings"
	"testing"
)

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

func TestLazyArrayRecordUpdatesPriorityAndExplainUnderConcurrency(t *testing.T) {
	cond := map[string]any{
		"scores": map[string]any{
			"$size":  map[string]any{"$gte": 3, "$lte": 5},
			"$every": map[string]any{"$gte": 60},
		},
		"tags": "ts",
		"reviews": map[string]any{
			"approved": true,
			"rating":   map[string]any{"$gte": 4},
		},
	}
	tree, err := Compile(cond)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	all := tree.root.(*allOf)
	var beforeChildren []int
	for _, c := range all.slice() {
		beforeChildren = append(beforeChildren, c.priority())
	}
	beforeRoot := tree.root.priority()
	if beforeRoot != 34 {
		t.Fatalf("expected initial priority 34 (children=%v), got %d", beforeChildren, beforeRoot)
	}

	rec := map[string]any{
		"candidate": "Alice",
		"tags":      []any{"go", "ts", "rust"},
		"scores":    []any{82, 91, 77},
		"reviews": []any{
			map[string]any{"reviewer": "r1", "approved": false, "rating": 5},
			map[string]any{"reviewer": "r2", "approved": true, "rating": 5},
			map[string]any{"reviewer": "r3", "approved": true, "rating": 4},
		},
	}

	// Trigger concurrent Match, Explain, and priority reads for -race coverage.
	done := make(chan struct{}, 4)
	for range 4 {
		go func() {
			if !tree.Match(rec) {
				t.Errorf("expected Alice to match")
			}
			_ = tree.root.priority()
			_ = tree.Explain()
			done <- struct{}{}
		}()
	}
	for range 4 {
		<-done
	}

	afterRoot := tree.root.priority()
	if afterRoot != 40 {
		t.Fatalf("expected updated priority 40 after lazy array build, got %d", afterRoot)
	}

	// Extension path (*extLiteralMatcher) also updates priority cleanly.
	extTree, err := Compile(map[string]any{"tags": "ts"}, WithTypeConverter(func(s string) any { return s }))
	if err != nil {
		t.Fatalf("ext Compile failed: %v", err)
	}
	if extTree.root.priority() != 3 {
		t.Fatalf("expected initial ext priority 3, got %d", extTree.root.priority())
	}
	if !extTree.Match(map[string]any{"tags": []any{"go", "ts"}}) {
		t.Fatal("expected ext match")
	}
	if extTree.root.priority() != 6 {
		t.Fatalf("expected updated ext priority 6, got %d", extTree.root.priority())
	}
}

func childFields(g *allOf) []string {
	var fields []string
	for _, c := range g.slice() {
		if fm, ok := c.(*fieldMatcher); ok {
			fields = append(fields, fm.field)
		}
	}
	return fields
}

func TestLazyArrayRecordResortsCompositeChildren(t *testing.T) {
	tree, err := Compile(map[string]any{
		"tags":  "ts",
		"age":   map[string]any{"$gte": 18},
		"items": map[string]any{"$size": map[string]any{"$gte": 1}},
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	root := tree.root.(*allOf)
	if got := strings.Join(childFields(root), ","); got != "tags,age,items" {
		t.Fatalf("expected initial order tags,age,items, got %s", got)
	}

	if !tree.Match(map[string]any{
		"tags":  []any{"go", "ts"},
		"age":   20,
		"items": []any{"x"},
	}) {
		t.Fatal("expected match")
	}

	if got := strings.Join(childFields(root), ","); got != "age,tags,items" {
		t.Fatalf("expected re-sorted order age,tags,items, got %s", got)
	}
	explain := tree.Explain()
	if strings.Index(explain, `Field("age")`) > strings.Index(explain, `Field("tags")`) {
		t.Fatalf("expected Explain to show age before tags, got:\n%s", explain)
	}
	matched, trace := tree.Trace(map[string]any{
		"tags":  []any{"ts"},
		"age":   10,
		"items": []any{"x"},
	})
	if matched {
		t.Fatal("expected mismatch on age=10")
	}
	if !strings.Contains(trace, "[SKIP] 2 remaining branch(es)") || strings.Contains(trace, `Field("tags")`) {
		t.Fatalf("expected age=10 to short-circuit before tags, got:\n%s", trace)
	}

	// $or also re-sorts children and short-circuits on the cheaper branch first.
	orTree, err := Compile(map[string]any{
		"$or": []any{
			map[string]any{"tags": "ts"},
			map[string]any{"age": map[string]any{"$gte": 18}},
		},
	})
	if err != nil {
		t.Fatalf("$or Compile failed: %v", err)
	}
	orNode := orTree.root.(*anyOf)
	if f0 := orNode.slice()[0].(*fieldMatcher).field; f0 != "tags" {
		t.Fatalf("expected initial $or first child tags, got %s", f0)
	}
	if !orTree.Match(map[string]any{"tags": []any{"go", "ts"}, "age": 10}) {
		t.Fatal("expected initial $or match")
	}
	if f0 := orNode.slice()[0].(*fieldMatcher).field; f0 != "age" {
		t.Fatalf("expected re-sorted $or first child age, got %s", f0)
	}

	// Wrapper chain ($not + nested array field) + extLiteralMatcher path re-sort cleanly under concurrency.
	wrappedTree, err := Compile(map[string]any{
		"matrix": map[string]any{"tags": map[string]any{"$not": "bad"}},
		"score":  map[string]any{"$gte": 0, "$lte": 100, "$ne": 50},
	}, WithTypeConverter(func(s string) any { return s }))
	if err != nil {
		t.Fatalf("wrapped Compile failed: %v", err)
	}
	wRoot := wrappedTree.root.(*allOf)
	if wRoot.slice()[0].(*fieldMatcher).field != "matrix" {
		t.Fatalf("expected initial matrix before score, got %s", wRoot.slice()[0].(*fieldMatcher).field)
	}
	done := make(chan struct{}, 4)
	for range 4 {
		go func() {
			if !wrappedTree.Match(map[string]any{
				"matrix": []any{map[string]any{"tags": []any{"good"}}},
				"score":  10,
			}) {
				t.Errorf("expected wrapped match")
			}
			_ = wrappedTree.Explain()
			done <- struct{}{}
		}()
	}
	for range 4 {
		<-done
	}
	if wRoot.slice()[0].(*fieldMatcher).field != "score" {
		t.Fatalf("expected re-sorted score before matrix, got %s", wRoot.slice()[0].(*fieldMatcher).field)
	}
}
