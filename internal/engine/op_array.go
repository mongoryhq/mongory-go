package engine

func init() {
	register("$elemMatch", buildElemMatch)
	register("$every", buildEvery)
	register("$size", buildSize)
}

// elemMatchMatcher matches an array with at least one element satisfying the
// condition.
type elemMatchMatcher struct {
	inner matcher
}

func buildElemMatch(condition any) (matcher, error) {
	inner, err := buildElementCondition("$elemMatch", condition)
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

// everyMatcher matches a non-empty array whose elements all satisfy the
// condition.
type everyMatcher struct {
	inner matcher
}

func buildEvery(condition any) (matcher, error) {
	inner, err := buildElementCondition("$every", condition)
	if err != nil {
		return nil, err
	}
	return everyMatcher{inner: inner}, nil
}

func (m everyMatcher) match(record any) bool {
	arr, ok := record.([]any)
	if !ok || len(arr) == 0 {
		return false
	}
	for _, e := range arr {
		if !m.inner.match(normalize(e)) {
			return false
		}
	}
	return true
}

func (m everyMatcher) priority() int { return 3 + m.inner.priority() }

func buildElementCondition(operator string, condition any) (matcher, error) {
	c, ok := condition.(map[string]any)
	if !ok {
		return nil, invalidCondition("%s needs an object, got %T", operator, condition)
	}
	return buildHashCondition(c)
}

// sizeMatcher matches an array whose length satisfies a literal condition,
// so both {"$size": 3} and {"$size": {"$gte": 1}} work.
type sizeMatcher struct {
	inner *literalMatcher
}

func buildSize(condition any) (matcher, error) {
	inner, err := buildLiteral(condition)
	if err != nil {
		return nil, err
	}
	return sizeMatcher{inner: inner}, nil
}

func (m sizeMatcher) match(record any) bool {
	arr, ok := record.([]any)
	return ok && m.inner.match(int64(len(arr)))
}

func (m sizeMatcher) priority() int { return 2 + m.inner.priority() }
