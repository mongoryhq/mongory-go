package engine

func init() {
	register("$eq", func(c any) (matcher, error) { return eqMatcher{condition: c}, nil })
}

// eqMatcher matches a value equal to the condition, or an array containing it.
type eqMatcher struct {
	condition any
}

func (m eqMatcher) match(record any) bool {
	if valuesEqual(record, m.condition) {
		return true
	}
	if arr, ok := record.([]any); ok {
		for _, e := range arr {
			if valuesEqual(e, m.condition) {
				return true
			}
		}
	}
	return false
}

func (eqMatcher) priority() int { return 1 }
