package mongory_test

import (
	"encoding/json"
	"math"
	"sync"
	"testing"

	"github.com/mongoryhq/mongory-go"
)

func TestExplainTreeAndTraceTree(t *testing.T) {
	t.Run("nil matcher safety", func(t *testing.T) {
		var nilM *mongory.Matcher
		if got := nilM.ExplainTree(); got != nil {
			t.Fatalf("nil ExplainTree = %#v, want nil", got)
		}
		if ok, tree := nilM.TraceTree(mongory.M{"age": 20}); ok || tree != nil {
			t.Fatalf("nil TraceTree = (%v, %#v), want (false, nil)", ok, tree)
		}
		var zero mongory.Matcher
		if got := zero.ExplainTree(); got != nil {
			t.Fatalf("zero ExplainTree = %#v, want nil", got)
		}
		if ok, tree := zero.TraceTree(mongory.M{"age": 20}); ok || tree != nil {
			t.Fatalf("zero TraceTree = (%v, %#v), want (false, nil)", ok, tree)
		}
	})

	t.Run("ExplainTree structure and priority order", func(t *testing.T) {
		m := mongory.MustCompile(mongory.M{
			"age":    mongory.M{"$gte": 18},
			"status": "active",
		})
		root := m.ExplainTree()
		if root == nil || root.Kind != "and" || root.Operator != "$and" || len(root.Children) != 2 {
			t.Fatalf("unexpected root ExplainNode: %#v", root)
		}
		// status ($eq, priority 1) sorts before age ($gte, priority 3)
		if root.Children[0].Kind != "field" || root.Children[0].Field != "status" {
			t.Fatalf("expected status first, got %#v", root.Children[0])
		}
		if root.Children[0].Children[0].Operator != "$eq" {
			t.Fatalf("expected $eq operator, got %#v", root.Children[0].Children[0])
		}
		if root.Children[1].Kind != "field" || root.Children[1].Field != "age" {
			t.Fatalf("expected age second, got %#v", root.Children[1])
		}
		if root.Children[1].Children[0].Operator != "$gte" {
			t.Fatalf("expected $gte operator, got %#v", root.Children[1].Children[0])
		}
		if _, err := json.Marshal(root); err != nil {
			t.Fatalf("json.Marshal(ExplainTree) failed: %v", err)
		}
	})

	t.Run("TraceTree PASS, FAIL, and short-circuit SKIP", func(t *testing.T) {
		m := mongory.MustCompile(mongory.M{
			"age":    mongory.M{"$gte": 18},
			"status": "active",
		})
		ok, tree := m.TraceTree(mongory.M{"age": 15, "status": "inactive"})
		if ok {
			t.Fatal("expected match=false")
		}
		if tree == nil || tree.Status != mongory.TraceFail || len(tree.Children) != 2 {
			t.Fatalf("unexpected TraceTree: %#v", tree)
		}
		if tree.Children[0].Field != "status" || tree.Children[0].Status != mongory.TraceFail {
			t.Fatalf("expected status fail, got %#v", tree.Children[0])
		}
		skip := tree.Children[1]
		if skip.Kind != "skip" || skip.Status != mongory.TraceSkip || skip.SkippedCount != 1 {
			t.Fatalf("expected skip node with skipped_count=1, got %#v", skip)
		}
	})

	t.Run("TraceTree array $elemMatch, $every, $size, missing vs nil", func(t *testing.T) {
		m := mongory.MustCompile(mongory.M{
			"scores": mongory.M{"$elemMatch": mongory.M{"$gt": 80}},
			"tags":   mongory.M{"$size": 2},
			"note":   mongory.M{"$exists": false},
		})
		ok, tree := m.TraceTree(mongory.M{
			"scores": mongory.A{50, 90, 95},
			"tags":   mongory.A{"go", "c"},
		})
		if !ok || tree == nil || tree.Status != mongory.TracePass {
			t.Fatalf("expected PASS trace tree, got ok=%v tree=%#v", ok, tree)
		}
		var sawElem, sawSkip, sawMissing bool
		var walk func(n *mongory.TraceNode)
		walk = func(n *mongory.TraceNode) {
			if n.Kind == "element" && n.Index != nil {
				sawElem = true
			}
			if n.Kind == "skip" && n.Status == mongory.TraceSkip && n.SkippedCount == 1 {
				sawSkip = true
			}
			if n.Field == "note" && n.Exists != nil && !*n.Exists {
				sawMissing = true
			}
			for i := range n.Children {
				walk(&n.Children[i])
			}
		}
		walk(tree)
		if !sawElem || !sawSkip || !sawMissing {
			b, _ := json.MarshalIndent(tree, "", "  ")
			t.Fatalf("expected elem=%v skip=%v missing=%v in:\n%s", sawElem, sawSkip, sawMissing, b)
		}
	})

	t.Run("TraceTree is always JSON-serializable for exotic and cyclic values", func(t *testing.T) {
		m := mongory.MustCompile(mongory.M{
			"num":  mongory.M{"$gt": 0},
			"self": mongory.M{"$exists": true},
		})
		cyclic := mongory.M{"num": math.NaN(), "inf": math.Inf(1), "fn": func() {}}
		cyclic["self"] = cyclic

		_, tree := m.TraceTree(cyclic)
		if tree == nil {
			t.Fatal("expected non-nil TraceTree")
		}
		data, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("json.Marshal failed on exotic/cyclic TraceTree: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("empty JSON output")
		}
	})

	t.Run("concurrent Match, ExplainTree, TraceTree under -race", func(t *testing.T) {
		m := mongory.MustCompile(mongory.M{
			"tags": mongory.M{"$in": mongory.A{"go", "c"}},
			"age":  mongory.M{"$gte": 18},
		})
		rec := mongory.M{"tags": mongory.A{"go"}, "age": 25}
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 50 {
					if !m.Match(rec) {
						t.Error("expected Match=true")
					}
					if m.ExplainTree() == nil {
						t.Error("expected non-nil ExplainTree")
					}
					if ok, tr := m.TraceTree(rec); !ok || tr == nil {
						t.Error("expected TraceTree=true")
					}
				}
			}()
		}
		wg.Wait()
	})
}
