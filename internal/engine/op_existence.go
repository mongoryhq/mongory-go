package engine

func init() {
	register("$exists", buildBoolOperator("$exists", func(record any, want bool) bool {
		return (record != missing) == want
	}))
	register("$present", buildBoolOperator("$present", func(record any, want bool) bool {
		return isPresent(record) == want
	}))
}

// boolMatcher implements operators whose condition is a boolean flag.
type boolMatcher struct {
	want bool
	test func(record any, want bool) bool
}

func buildBoolOperator(operator string, test func(record any, want bool) bool) builderFunc {
	return func(c any) (matcher, error) {
		want, ok := c.(bool)
		if !ok {
			return nil, invalidCondition("%s needs a boolean, got %T", operator, c)
		}
		return boolMatcher{want: want, test: test}, nil
	}
}

func (m boolMatcher) match(record any) bool { return m.test(record, m.want) }
func (boolMatcher) priority() int           { return 2 }

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
