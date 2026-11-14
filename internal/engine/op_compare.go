package engine

func init() {
	register("$eq", func(c any) (matcher, error) { return eqMatcher{condition: c}, nil })
	register("$ne", func(c any) (matcher, error) { return neMatcher{condition: c}, nil })
	register("$gt", buildOrder(func(c int) bool { return c > 0 }))
	register("$gte", buildOrder(func(c int) bool { return c >= 0 }))
	register("$lt", buildOrder(func(c int) bool { return c < 0 }))
	register("$lte", buildOrder(func(c int) bool { return c <= 0 }))
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

// neMatcher is the strict negation of eqMatcher.
type neMatcher struct {
	condition any
}

func (m neMatcher) match(record any) bool { return !eqMatcher(m).match(record) }
func (neMatcher) priority() int           { return 1 }

// orderMatcher implements $gt/$gte/$lt/$lte; incomparable pairs never match.
type orderMatcher struct {
	condition any
	accept    func(cmp int) bool
}

func buildOrder(accept func(cmp int) bool) builderFunc {
	return func(c any) (matcher, error) {
		return orderMatcher{condition: c, accept: accept}, nil
	}
}

func (m orderMatcher) match(record any) bool {
	cmp, ok := compareValues(record, m.condition)
	return ok && m.accept(cmp)
}

func (orderMatcher) priority() int { return 3 }
