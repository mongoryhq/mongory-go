package engine

import (
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// matcher is one node of the compiled tree. match receives a canonical value
// (or missing); matchReflect evaluates an unnormalized reflect.Value directly
// without allocating intermediate maps or []any slices.
type matcher interface {
	match(record any) bool
	matchReflect(rv reflect.Value) bool
	// priority orders sibling matchers so cheaper ones run first.
	priority() int
	explain(buf *strings.Builder, prefix string, isLast, isRoot bool)
	trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool
}

func matchRaw(m matcher, v any) bool {
	if norm, ok := normalizeFast(v); ok {
		return m.match(norm)
	}
	return m.matchReflect(reflect.ValueOf(v))
}

func matchReflectValue(m matcher, rv reflect.Value) bool {
	if rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return m.match(nil)
		}
		return matchRaw(m, rv.Interface())
	}
	return m.matchReflect(rv)
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

func (m allOf) matchReflect(rv reflect.Value) bool {
	for _, c := range m {
		if !c.matchReflect(rv) {
			return false
		}
	}
	return true
}

func (m allOf) priority() int { return 1 + sumPriority(m) }
func (m allOf) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	if len(m) == 0 {
		writeTreeLine(buf, prefix, isLast, isRoot, "And (empty)")
		return
	}
	explainBranch(buf, prefix, isLast, isRoot, "And", m...)
}
func (m allOf) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceGroup(record, buf, prefix, isLast, isRoot, "And", m, false)
}

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

func (m anyOf) matchReflect(rv reflect.Value) bool {
	for _, c := range m {
		if c.matchReflect(rv) {
			return true
		}
	}
	return false
}

func (m anyOf) priority() int { return 1 + sumPriority(m) }
func (m anyOf) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	if len(m) == 0 {
		writeTreeLine(buf, prefix, isLast, isRoot, "Or (empty)")
		return
	}
	explainBranch(buf, prefix, isLast, isRoot, "Or", m...)
}
func (m anyOf) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceGroup(record, buf, prefix, isLast, isRoot, "Or", m, true)
}

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
	for _, key := range sortedKeys(condition) {
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

func (m *literalMatcher) matchReflect(rv reflect.Value) bool {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return m.dispatched.match(nil)
	}
	switch rv.Kind() {
	case reflect.String:
		if rv.Type() == jsonNumberType {
			return m.match(parseJSONNumberString(rv.String()))
		}
	case reflect.Slice:
		if !isKeyValueStruct(rv.Type().Elem()) {
			m.arrayOnce.Do(func() { m.arrayRecord, _ = buildArrayRecord(m.condition) })
			return m.arrayRecord != nil && m.arrayRecord.matchReflect(rv)
		}
	case reflect.Array:
		if rv.Type().Elem().Kind() != reflect.Uint8 {
			m.arrayOnce.Do(func() { m.arrayRecord, _ = buildArrayRecord(m.condition) })
			return m.arrayRecord != nil && m.arrayRecord.matchReflect(rv)
		}
	}
	return m.dispatched.matchReflect(rv)
}

func (m *literalMatcher) priority() int { return 1 + m.dispatched.priority() }
func (m *literalMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	m.dispatched.explain(buf, prefix, isLast, isRoot)
}
func (m *literalMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	if _, ok := record.([]any); ok {
		m.arrayOnce.Do(func() { m.arrayRecord, _ = buildArrayRecord(m.condition) })
		if m.arrayRecord == nil {
			writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] ArrayRecord | value=%s", formatValue(record)))
			return false
		}
		return m.arrayRecord.trace(record, buf, prefix, isLast, isRoot)
	}
	return m.dispatched.trace(record, buf, prefix, isLast, isRoot)
}

// nullMatcher implements a literal null condition: missing or explicit nil.
type nullMatcher struct{}

func (nullMatcher) match(record any) bool              { return record == nil || record == missing }
func (nullMatcher) matchReflect(rv reflect.Value) bool { _, ok := unwrapReflect(rv); return !ok }
func (nullMatcher) priority() int                      { return 2 }
func (nullMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, "Eq(nil) | NotExists")
}
func (m nullMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, "Eq(nil) | NotExists")
}

// fieldMatcher digs one key out of a map (or one index out of an array) and
// hands the value to its literal matcher.
type fieldMatcher struct {
	field    string
	fieldVal reflect.Value
	index    int
	isIndex  bool
	literal  *literalMatcher
	slot     atomic.Pointer[fieldSlot]
}

type fieldSlot struct {
	typ           reflect.Type
	found         bool
	isScalar      bool // time.Time or implements timeValuer
	isRegex       bool
	field         structField
	hasInlineMaps bool
}

func buildField(field string, condition any) (matcher, error) {
	literal, err := buildLiteral(condition)
	if err != nil {
		return nil, err
	}
	index, isIndex := parseIndexKey(field)
	return &fieldMatcher{
		field:    field,
		fieldVal: reflect.ValueOf(field),
		index:    index,
		isIndex:  isIndex,
		literal:  literal,
	}, nil
}

func (m *fieldMatcher) resolveStructSlot(t reflect.Type) (*fieldSlot, *structInfo) {
	if slot := m.slot.Load(); slot != nil && slot.typ == t {
		return slot, nil
	}
	info := getStructInfo(t)
	sf, found := info.byName[m.field]
	slot := &fieldSlot{
		typ:           t,
		found:         found,
		isScalar:      info.isTimeValuer,
		isRegex:       info.isRegex,
		field:         sf,
		hasInlineMaps: len(info.inlineMaps) > 0,
	}
	if m.slot.Load() == nil {
		m.slot.CompareAndSwap(nil, slot)
	}
	return slot, info
}

func (m *fieldMatcher) extract(record any) (any, bool) {
	switch r := record.(type) {
	case map[string]any:
		v, ok := r[m.field]
		if !ok {
			v = missing
		}
		return v, true
	case []any:
		if m.isIndex {
			i := m.index
			if i < 0 {
				i += len(r)
			}
			if i >= 0 && i < len(r) {
				return r[i], true
			}
		}
		return missing, true
	default:
		return nil, false
	}
}

func (m *fieldMatcher) match(record any) bool {
	value, ok := m.extract(record)
	if !ok {
		return false
	}
	return matchRaw(m.literal, value)
}

func (m *fieldMatcher) matchReflect(rv reflect.Value) bool {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return false
	}
	switch rv.Kind() {
	case reflect.Struct:
		return m.matchStructField(rv)
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return false
		}
		elem := rv.MapIndex(m.fieldVal)
		if !elem.IsValid() {
			return m.literal.match(missing)
		}
		return matchReflectValue(m.literal, elem)
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return false
		}
		return m.matchSliceIndex(rv)
	case reflect.Slice:
		if isKeyValueStruct(rv.Type().Elem()) {
			for i := range rv.Len() {
				entry := rv.Index(i)
				if entry.Field(0).String() == m.field {
					return matchRaw(m.literal, entry.Field(1).Interface())
				}
			}
			return m.literal.match(missing)
		}
		return m.matchSliceIndex(rv)
	}
	return false
}

func (m *fieldMatcher) matchStructField(rv reflect.Value) bool {
	slot, info := m.resolveStructSlot(rv.Type())
	if slot.isScalar {
		return false
	}
	if slot.isRegex {
		return m.match(normalizeReflect(rv))
	}
	if slot.found {
		if slot.field.singleIndex >= 0 {
			return matchReflectValue(m.literal, rv.Field(slot.field.singleIndex))
		}
		if fv, exists := fieldValue(rv, slot.field); exists {
			return matchReflectValue(m.literal, fv)
		}
	}
	if slot.hasInlineMaps {
		if info == nil {
			info = getStructInfo(rv.Type())
		}
		for _, im := range info.inlineMaps {
			if mv, exists := fieldValue(rv, im); exists && !mv.IsNil() {
				if elem := mv.MapIndex(m.fieldVal); elem.IsValid() {
					return matchReflectValue(m.literal, elem)
				}
			}
		}
	}
	return m.literal.match(missing)
}

func (m *fieldMatcher) matchSliceIndex(rv reflect.Value) bool {
	if m.isIndex {
		i := m.index
		if i < 0 {
			i += rv.Len()
		}
		if i >= 0 && i < rv.Len() {
			return matchReflectValue(m.literal, rv.Index(i))
		}
	}
	return m.literal.match(missing)
}

func (m *fieldMatcher) priority() int { return 1 + m.literal.priority() }
func (m *fieldMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	explainBranch(buf, prefix, isLast, isRoot, fmt.Sprintf("Field(%q)", m.field), m.literal)
}
func (m *fieldMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	value, ok := m.extract(record)
	if !ok {
		writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] Field(%q) | value=%s", m.field, formatValue(record)))
		return false
	}
	return traceWrapped(normalize(value), buf, prefix, isLast, isRoot, fmt.Sprintf("Field(%q)", m.field), m.literal, false)
}

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
	for _, key := range sortedKeys(condition) {
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
