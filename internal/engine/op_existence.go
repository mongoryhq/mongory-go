package engine

import (
	"fmt"
	"strings"
)

func init() {
	register("$exists", buildBoolOperator("Exists", func(record any, want bool) bool {
		return (record != missing) == want
	}))
	register("$present", buildBoolOperator("Present", func(record any, want bool) bool {
		return isPresent(record) == want
	}))
}

// boolMatcher implements operators whose condition is a boolean flag.
type boolMatcher struct {
	op   string
	want bool
	test func(record any, want bool) bool
}

func buildBoolOperator(op string, test func(record any, want bool) bool) builderFunc {
	return func(c any) (matcher, error) {
		want, ok := c.(bool)
		if !ok {
			return nil, invalidCondition("$%s needs a boolean, got %T", strings.ToLower(op), c)
		}
		return boolMatcher{op: op, want: want, test: test}, nil
	}
}

func (m boolMatcher) match(record any) bool { return m.test(record, m.want) }
func (boolMatcher) priority() int           { return 2 }
func (m boolMatcher) label() string         { return fmt.Sprintf("%s(%t)", m.op, m.want) }
func (m boolMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m boolMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}

// isPresent is false for missing, nil, false, and empty strings, arrays and
// objects; true for everything else, including 0.
func isPresent(v any) bool {
	switch x := v.(type) {
	case missingKey, nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}
