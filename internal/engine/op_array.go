package engine

func init() {
	register("$elemMatch", buildElemMatch)
}

// elemMatchMatcher matches an array with at least one element satisfying the
// condition.
type elemMatchMatcher struct {
	inner matcher
}

func buildElemMatch(condition any) (matcher, error) {
	c, ok := condition.(map[string]any)
	if !ok {
		return nil, invalidCondition("$elemMatch needs an object, got %T", condition)
	}
	inner, err := buildHashCondition(c)
	if err != nil {
		return nil, err
	}
	return elemMatchMatcher{inner: inner}, nil
}

func (m elemMatchMatcher) match(record any) bool {
	arr, ok := record.([]any)
	if !ok {
		return false
	}
	for _, e := range arr {
		if m.inner.match(normalize(e)) {
			return true
		}
	}
	return false
}

func (m elemMatchMatcher) priority() int { return 3 + m.inner.priority() }
