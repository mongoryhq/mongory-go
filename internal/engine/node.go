package engine

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// matcher is one node of the compiled tree. match receives a value already
// passed through normalize.
type matcher interface {
	match(record any) bool
	// priority orders sibling matchers so cheaper ones run first.
	priority() int
}

type builderFunc func(condition any) (matcher, error)

var operators = map[string]builderFunc{}

// register is only called from init functions.
func register(operator string, fn builderFunc) {
	if _, dup := operators[operator]; dup {
		panic("mongory: duplicate operator " + operator)
	}
	operators[operator] = fn
}

func invalidCondition(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidCondition}, args...)...)
}

// allOf matches when every child matches; empty matches anything.
type allOf []matcher

func (m allOf) match(record any) bool {
	for _, c := range m {
		if !c.match(record) {
			return false
		}
	}
	return true
}

func (m allOf) priority() int { return 1 + sumPriority(m) }

// anyOf matches when some child matches.
type anyOf []matcher

func (m anyOf) match(record any) bool {
	for _, c := range m {
		if c.match(record) {
			return true
		}
	}
	return false
}

func (m anyOf) priority() int { return 1 + sumPriority(m) }

func sumPriority(ms []matcher) int {
	total := 0
	for _, c := range ms {
		total += c.priority()
	}
	return total
}

func sortByPriority(ms []matcher) {
	slices.SortStableFunc(ms, func(a, b matcher) int { return a.priority() - b.priority() })
}

// buildHashCondition ANDs one matcher per key: registered operators build
// themselves, an unknown "$" key is rejected as MongoDB does, and any other
// key is a field lookup. "$options" is consumed by its sibling "$regex".
func buildHashCondition(condition map[string]any) (matcher, error) {
	children := make([]matcher, 0, len(condition))
	for _, key := range slices.Sorted(maps.Keys(condition)) {
		var (
			child matcher
			err   error
		)
		build, isOperator := operators[key]
		switch {
		case key == "$options":
			if _, ok := condition["$regex"]; ok {
				continue
			}
			err = invalidCondition("$options needs a sibling $regex")
		case key == "$regex":
			child, err = buildRegex(condition[key], condition["$options"])
		case isOperator:
			child, err = build(condition[key])
		case strings.HasPrefix(key, "$"):
			err = invalidCondition("unknown operator %s", key)
		default:
			child, err = buildField(key, condition[key])
		}
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}
	if len(children) == 1 {
		return children[0], nil
	}
	sortByPriority(children)
	return allOf(children), nil
}

// literalMatcher matches a field value against a condition that is a plain
// value or a nested condition object. Array values take the MongoDB implicit
// array path instead.
type literalMatcher struct {
	condition  any
	dispatched matcher

	arrayOnce   sync.Once
	arrayRecord matcher
}

func buildLiteral(condition any) (*literalMatcher, error) {
	var (
		dispatched matcher
		err        error
	)
	switch c := condition.(type) {
	case map[string]any:
		dispatched, err = buildHashCondition(c)
	case *regexp.Regexp:
		dispatched = regexMatcher{re: c}
	case nil:
		dispatched = nullMatcher{}
	default:
		dispatched = eqMatcher{condition: c}
	}
	if err != nil {
		return nil, err
	}
	return &literalMatcher{condition: condition, dispatched: dispatched}, nil
}

func (m *literalMatcher) match(record any) bool {
	if _, ok := record.([]any); ok {
		// Built lazily: building it eagerly at every level doubles the tree
		// per nesting depth.
		m.arrayOnce.Do(func() { m.arrayRecord, _ = buildArrayRecord(m.condition) })
		return m.arrayRecord != nil && m.arrayRecord.match(record)
	}
	return m.dispatched.match(record)
}

func (m *literalMatcher) priority() int { return 1 + m.dispatched.priority() }

// nullMatcher implements a literal null condition: missing or explicit nil.
type nullMatcher struct{}

func (nullMatcher) match(record any) bool { return record == nil || record == missing }
func (nullMatcher) priority() int         { return 2 }

// fieldMatcher digs one key out of a map (or one index out of an array) and
// hands the value to its literal matcher.
type fieldMatcher struct {
	field   string
	index   int
	isIndex bool
	literal *literalMatcher
}

func buildField(field string, condition any) (matcher, error) {
	literal, err := buildLiteral(condition)
	if err != nil {
		return nil, err
	}
	index, isIndex := parseIndexKey(field)
	return &fieldMatcher{field: field, index: index, isIndex: isIndex, literal: literal}, nil
}

func (m *fieldMatcher) match(record any) bool {
	var value any
	switch r := record.(type) {
	case map[string]any:
		v, ok := r[m.field]
		if !ok {
			v = missing
		}
		value = v
	case []any:
		value = missing
		if m.isIndex {
			i := m.index
			if i < 0 {
				i += len(r)
			}
			if i >= 0 && i < len(r) {
				value = r[i]
			}
		}
	default:
		return false
	}
	return m.literal.match(normalize(value))
}

func (m *fieldMatcher) priority() int { return 1 + m.literal.priority() }

// parseIndexKey accepts canonical base-10 integers only: "01", "-0" and "+1"
// are field names, not array indexes.
func parseIndexKey(key string) (int, bool) {
	n, err := strconv.Atoi(key)
	if err != nil || strconv.Itoa(n) != key {
		return 0, false
	}
	return n, true
}

// buildArrayRecord applies a literal condition to an array value.
func buildArrayRecord(condition any) (matcher, error) {
	var children []matcher
	if _, ok := condition.([]any); ok {
		children = append(children, eqMatcher{condition: condition})
	}
	var (
		child matcher
		err   error
	)
	switch c := condition.(type) {
	case map[string]any:
		child, err = buildHashCondition(splitArrayCondition(c))
	case *regexp.Regexp:
		child, err = buildElemMatch(map[string]any{"$regex": c})
	default:
		child, err = buildElemMatch(map[string]any{"$eq": c})
	}
	if err != nil {
		return nil, err
	}
	children = append(children, child)
	if len(children) == 1 {
		return children[0], nil
	}
	sortByPriority(children)
	return anyOf(children), nil
}

// elementOperators apply per element when used directly on an array field.
var elementOperators = map[string]bool{"$gt": true, "$gte": true, "$lt": true, "$lte": true, "$regex": true}

// splitArrayCondition keeps index keys and array-level operators on the array
// itself and groups everything else (including "$options") into an implicit
// $elemMatch.
func splitArrayCondition(condition map[string]any) map[string]any {
	parsed := map[string]any{}
	elem := map[string]any{}
	for _, key := range slices.Sorted(maps.Keys(condition)) {
		value := condition[key]
		_, isIndex := parseIndexKey(key)
		_, isOperator := operators[key]
		switch {
		case isIndex:
			parsed[key] = value
		case key == "$elemMatch":
			if m, ok := value.(map[string]any); ok {
				maps.Copy(elem, m)
			} else {
				parsed[key] = value
			}
		case isOperator:
			if isElementOperator(key, value) {
				elem[key] = value
			} else {
				parsed[key] = value
			}
		default:
			elem[key] = value
		}
	}
	if len(elem) > 0 {
		parsed["$elemMatch"] = elem
	}
	return parsed
}

func isElementOperator(key string, value any) bool {
	if key == "$eq" {
		_, isArray := value.([]any)
		return !isArray
	}
	return elementOperators[key]
}
