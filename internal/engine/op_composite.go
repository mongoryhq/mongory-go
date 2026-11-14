package engine

func init() {
	register("$and", func(c any) (matcher, error) {
		children, err := buildConditionList("$and", c)
		return allOf(children), err
	})
	register("$or", func(c any) (matcher, error) {
		children, err := buildConditionList("$or", c)
		return anyOf(children), err
	})
	register("$not", buildNot)
}

// buildConditionList builds one hash condition per element; an empty list
// yields an empty allOf (matches anything) or anyOf (matches nothing).
func buildConditionList(operator string, condition any) ([]matcher, error) {
	list, ok := condition.([]any)
	if !ok {
		return nil, invalidCondition("%s needs an array, got %T", operator, condition)
	}
	children := make([]matcher, 0, len(list))
	for _, item := range list {
		sub, ok := item.(map[string]any)
		if !ok {
			return nil, invalidCondition("%s needs an array of objects, got %T", operator, item)
		}
		child, err := buildHashCondition(sub)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	sortByPriority(children)
	return children, nil
}

// notMatcher negates a literal condition, so it also matches missing fields.
type notMatcher struct {
	inner *literalMatcher
}

func buildNot(condition any) (matcher, error) {
	inner, err := buildLiteral(condition)
	if err != nil {
		return nil, err
	}
	return notMatcher{inner: inner}, nil
}

func (m notMatcher) match(record any) bool { return !m.inner.match(record) }
func (m notMatcher) priority() int         { return 1 + m.inner.priority() }
