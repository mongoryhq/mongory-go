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

type builderFunc func(ctx *compileCtx, condition any) (matcher, error)

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

func limitExceeded(format string, args ...any) error {
	return fmt.Errorf("%w: %w: "+format, append([]any{ErrInvalidCondition, ErrLimitExceeded}, args...)...)
}

type compileCtx struct {
	maxDepth  int
	maxNodes  int
	depth     int
	nodes     int
	conv      *converterRegistry
	getters   *getterRegistry
	customOps map[string]Operator
}

func newCompileCtx(cfg CompileConfig) *compileCtx {
	return &compileCtx{
		maxDepth:  cfg.MaxDepth,
		maxNodes:  cfg.MaxNodes,
		conv:      cfg.converters,
		getters:   cfg.getters,
		customOps: cfg.operators,
	}
}

func (c *compileCtx) withoutLimits() *compileCtx {
	if c == nil || (c.conv == nil && c.getters == nil && len(c.customOps) == 0) {
		return nil
	}
	return &compileCtx{
		conv:      c.conv,
		getters:   c.getters,
		customOps: c.customOps,
	}
}

func (c *compileCtx) convOrNil() *converterRegistry {
	if c == nil {
		return nil
	}
	return c.conv
}

func (c *compileCtx) customOp(key string) (Operator, bool) {
	if c == nil || len(c.customOps) == 0 {
		return Operator{}, false
	}
	op, ok := c.customOps[key]
	return op, ok
}

func (c *compileCtx) addNode() error {
	if c == nil || c.maxNodes <= 0 {
		return nil
	}
	if c.nodes >= c.maxNodes {
		return limitExceeded("matcher node count exceeds max nodes %d", c.maxNodes)
	}
	c.nodes++
	return nil
}

func addNodeMatcher(ctx *compileCtx, m matcher) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *compileCtx) enter() (int, error) {
	if c == nil {
		return 0, nil
	}
	prev := c.depth
	c.depth++
	if err := checkDepthLimit(c.depth, c.maxDepth); err != nil {
		return prev, err
	}
	return prev, nil
}

func (c *compileCtx) leave(prev int) {
	if c != nil {
		c.depth = prev
	}
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

// buildHashCondition ANDs one matcher per key: per-compile custom operators,
// registered built-in operators, unknown "$" key rejection, or field lookups.
// "$options" is consumed by its sibling "$regex". Nested non-empty allOf nodes
// are flattened so priority sorting operates across the combined conjunction.
func buildHashCondition(ctx *compileCtx, condition map[string]any) (matcher, error) {
	children := make([]matcher, 0, len(condition))
	rawCount := 0
	for _, key := range sortedKeys(condition) {
		var (
			child matcher
			err   error
		)
		customOp, isCustom := ctx.customOp(key)
		build, isOperator := operators[key]
		switch {
		case isCustom:
			var prev int
			if prev, err = ctx.enter(); err == nil {
				child, err = buildCustomOp(ctx, key, customOp, condition[key])
				ctx.leave(prev)
			}
		case key == "$options":
			if _, ok := condition["$regex"]; ok {
				continue
			}
			err = invalidCondition("$options needs a sibling $regex")
		case key == "$regex":
			var prev int
			if prev, err = ctx.enter(); err == nil {
				child, err = buildRegex(ctx, condition[key], condition["$options"])
				ctx.leave(prev)
			}
		case isOperator:
			var prev int
			if prev, err = ctx.enter(); err == nil {
				child, err = build(ctx, condition[key])
				ctx.leave(prev)
			}
		case strings.HasPrefix(key, "$"):
			err = invalidCondition("unknown operator %s", key)
		default:
			var prev int
			if prev, err = ctx.enter(); err == nil {
				child, err = buildField(ctx, key, condition[key])
				ctx.leave(prev)
			}
		}
		if err != nil {
			return nil, err
		}
		rawCount++
		if sub, ok := child.(allOf); ok {
			children = append(children, sub...)
			continue
		}
		children = append(children, child)
	}
	if rawCount == 1 {
		if len(children) == 1 {
			return children[0], nil
		}
		sortByPriority(children)
		return allOf(children), nil
	}
	if err := ctx.addNode(); err != nil {
		return nil, err
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

func containsCustomOp(m matcher) bool {
	switch x := m.(type) {
	case customOpMatcher:
		return true
	case *extLiteralMatcher:
		return x.hasCustomOp
	case allOf:
		for _, c := range x {
			if containsCustomOp(c) {
				return true
			}
		}
	case anyOf:
		for _, c := range x {
			if containsCustomOp(c) {
				return true
			}
		}
	case notMatcher:
		return containsCustomOp(x.inner)
	case elemMatchMatcher:
		return x.raw
	case everyMatcher:
		return x.raw
	}
	return false
}

func buildLiteral(ctx *compileCtx, condition any) (matcher, error) {
	var (
		dispatched matcher
		err        error
	)
	conv := ctx.convOrNil()
	switch c := condition.(type) {
	case map[string]any:
		dispatched, err = buildHashCondition(ctx, c)
	case *regexp.Regexp:
		m, merr := addNodeMatcher(ctx, regexMatcher{re: c})
		if merr == nil && conv != nil {
			m = convValueMatcher{conv: conv, inner: m}
		}
		dispatched, err = m, merr
	case nil:
		dispatched, err = addNodeMatcher(ctx, nullMatcher{})
	default:
		if conv != nil {
			dispatched, err = addNodeMatcher(ctx, eqConvMatcher{condition: c, conv: conv})
		} else {
			dispatched, err = addNodeMatcher(ctx, eqMatcher{condition: c})
		}
	}
	if err != nil {
		return nil, err
	}
	extCtx := ctx.withoutLimits()
	if extCtx == nil {
		return &literalMatcher{condition: condition, dispatched: dispatched}, nil
	}
	hasCustom := len(extCtx.customOps) > 0 && containsCustomOp(dispatched)
	return &extLiteralMatcher{
		condition:   condition,
		dispatched:  dispatched,
		arrayCtx:    extCtx,
		hasCustomOp: hasCustom,
		hasConv:     conv != nil,
	}, nil
}

func (m *literalMatcher) ensureArrayRecord() {
	m.arrayOnce.Do(func() { m.arrayRecord, _ = buildArrayRecord(m.condition, nil) })
}

func evalMatcherOnRaw(m matcher, v any) bool {
	switch x := m.(type) {
	case *extLiteralMatcher:
		if _, ok := v.([]any); ok {
			x.ensureArrayRecord()
			return x.arrayRecord != nil && evalMatcherOnRaw(x.arrayRecord, v)
		}
		if _, ok := v.(missingKey); !ok && v != nil {
			rv := reflect.ValueOf(v)
			if unwrapped, ok := unwrapReflect(rv); ok && isReflectList(unwrapped) {
				x.ensureArrayRecord()
				return x.arrayRecord != nil && x.arrayRecord.matchReflect(rv)
			}
		}
		return evalMatcherOnRaw(x.dispatched, v)
	case customOpMatcher:
		return x.match(v)
	case allOf:
		for _, c := range x {
			if !evalMatcherOnRaw(c, v) {
				return false
			}
		}
		return true
	case anyOf:
		for _, c := range x {
			if evalMatcherOnRaw(c, v) {
				return true
			}
		}
		return false
	case notMatcher:
		return !evalMatcherOnRaw(x.inner, v)
	case elemMatchMatcher:
		return evalListElementsRaw(v, x.inner, true)
	case everyMatcher:
		return evalListElementsRaw(v, x.inner, false)
	default:
		return matchRaw(m, v)
	}
}

func evalListElementsRaw(v any, inner matcher, stopOn bool) bool {
	if arr, ok := v.([]any); ok {
		if len(arr) == 0 {
			return false
		}
		for _, e := range arr {
			if evalMatcherOnRaw(inner, e) == stopOn {
				return stopOn
			}
		}
		return !stopOn
	}
	if v == nil {
		return false
	}
	if _, isMissing := v.(missingKey); isMissing {
		return false
	}
	return matchListElementsReflect(reflect.ValueOf(v), inner, true, stopOn)
}

func (m *literalMatcher) match(record any) bool {
	if _, ok := record.([]any); ok {
		// Built lazily: building it eagerly at every level doubles the tree
		// per nesting depth.
		m.ensureArrayRecord()
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
			m.ensureArrayRecord()
			return m.arrayRecord != nil && m.arrayRecord.matchReflect(rv)
		}
	case reflect.Array:
		if rv.Type().Elem().Kind() != reflect.Uint8 {
			m.ensureArrayRecord()
			return m.arrayRecord != nil && m.arrayRecord.matchReflect(rv)
		}
	}
	return m.dispatched.matchReflect(rv)
}

func (m *literalMatcher) priority() int { return 1 + m.dispatched.priority() }
func (m *literalMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	m.dispatched.explain(buf, prefix, isLast, isRoot)
}
func isArrayRecordValue(v any) bool {
	if _, ok := v.([]any); ok {
		return true
	}
	if v == nil {
		return false
	}
	if _, ok := v.(missingKey); ok {
		return false
	}
	rv, ok := unwrapReflect(reflect.ValueOf(v))
	return ok && isReflectList(rv)
}

func (m *literalMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	if isArrayRecordValue(record) {
		m.ensureArrayRecord()
		if m.arrayRecord == nil {
			writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] ArrayRecord | value=%s", formatTraceValue(record)))
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
	literal  matcher
	slot     atomic.Pointer[fieldSlot]
}

type fieldSlot struct {
	typ           reflect.Type
	found         bool
	isScalar      bool // time.Time or implements timeValuer
	isRegex       bool
	field         structField
	hasInlineMaps bool
	getter        *boundFieldGetter
	resolvers     []boundFieldResolver
}

func buildField(ctx *compileCtx, field string, condition any) (matcher, error) {
	if err := ctx.addNode(); err != nil {
		return nil, err
	}
	literal, err := buildLiteral(ctx, condition)
	if err != nil {
		return nil, err
	}
	index, isIndex := parseIndexKey(field)
	fm := &fieldMatcher{
		field:    field,
		fieldVal: reflect.ValueOf(field),
		index:    index,
		isIndex:  isIndex,
		literal:  literal,
	}
	var getters *getterRegistry
	if ctx != nil {
		getters = ctx.getters
	}
	extLit, _ := literal.(*extLiteralMatcher)
	raw := extLit != nil && extLit.hasCustomOp
	if getters != nil || raw {
		return &extFieldMatcher{
			fieldMatcher: fm,
			getters:      getters,
			raw:          raw,
		}, nil
	}
	return fm, nil
}

func (m *fieldMatcher) buildSlot(t reflect.Type, getters *getterRegistry) (*fieldSlot, *structInfo) {
	slot := &fieldSlot{typ: t}
	var info *structInfo
	if t.Kind() == reflect.Struct {
		info = getStructInfo(t)
		sf, found := info.byName[m.field]
		slot.found = found
		slot.isScalar = info.isTimeValuer
		slot.isRegex = info.isRegex
		slot.field = sf
		slot.hasInlineMaps = len(info.inlineMaps) > 0
	}
	if getters != nil {
		slot.getter, slot.resolvers = getters.resolveFor(t, m.field)
	}
	return slot, info
}

func (m *fieldMatcher) resolveStructSlot(t reflect.Type) (*fieldSlot, *structInfo) {
	if slot := m.slot.Load(); slot != nil && slot.typ == t {
		return slot, nil
	}
	slot, info := m.buildSlot(t, nil)
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

func elemInterface(v reflect.Value) (any, bool) {
	if !v.IsValid() || !v.CanInterface() {
		return nil, false
	}
	return v.Interface(), true
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

func (m *fieldMatcher) extractTraceValueWithGetters(record any, getters *getterRegistry) (any, bool) {
	if v, ok := m.extract(record); ok {
		return v, true
	}
	if record == nil {
		return nil, false
	}
	if _, isMissing := record.(missingKey); isMissing {
		return nil, false
	}
	rv, unwrapped := unwrapReflect(reflect.ValueOf(record))
	if !unwrapped {
		return nil, false
	}
	var cachedSlot *fieldSlot
	var cachedInfo *structInfo
	if getters != nil {
		slot, info := m.buildSlot(rv.Type(), getters)
		cachedSlot = slot
		cachedInfo = info
		if slot.getter != nil {
			return slot.getter.invoke(rv, slot.getter.needPtr), true
		}
		for _, r := range slot.resolvers {
			if val, handled := r.invoke(rv, r.needPtr, m.field); handled {
				return val, true
			}
		}
	}
	switch rv.Kind() {
	case reflect.Struct:
		slot, info := cachedSlot, cachedInfo
		if slot == nil {
			slot, info = m.resolveStructSlot(rv.Type())
		}
		if slot.isScalar {
			return nil, false
		}
		if slot.isRegex {
			return m.extract(normalizeReflect(rv))
		}
		if slot.found {
			if slot.field.singleIndex >= 0 {
				v, ok := elemInterface(rv.Field(slot.field.singleIndex))
				return v, ok
			}
			if fv, exists := fieldValue(rv, slot.field); exists {
				v, ok := elemInterface(fv)
				return v, ok
			}
		}
		if slot.hasInlineMaps {
			if info == nil {
				info = getStructInfo(rv.Type())
			}
			for _, im := range info.inlineMaps {
				if mv, exists := fieldValue(rv, im); exists && !mv.IsNil() {
					if elem := mv.MapIndex(m.fieldVal); elem.IsValid() {
						v, ok := elemInterface(elem)
						return v, ok
					}
				}
			}
		}
		return missing, true
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil, false
		}
		elem := rv.MapIndex(m.fieldVal)
		if !elem.IsValid() {
			return missing, true
		}
		v, ok := elemInterface(elem)
		return v, ok
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return nil, false
		}
		return m.extractTraceSliceIndex(rv), true
	case reflect.Slice:
		if isKeyValueStruct(rv.Type().Elem()) {
			for i := range rv.Len() {
				entry := rv.Index(i)
				if entry.Field(0).String() == m.field {
					v, ok := elemInterface(entry.Field(1))
					return v, ok
				}
			}
			return missing, true
		}
		return m.extractTraceSliceIndex(rv), true
	}
	return nil, false
}

func (m *fieldMatcher) extractTraceSliceIndex(rv reflect.Value) any {
	if m.isIndex {
		i := m.index
		if i < 0 {
			i += rv.Len()
		}
		if i >= 0 && i < rv.Len() {
			if v, ok := elemInterface(rv.Index(i)); ok {
				return v
			}
		}
	}
	return missing
}

func (m *fieldMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	value, ok := m.extractTraceValueWithGetters(record, nil)
	if !ok {
		writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[FAIL] Field(%q) | value=%s", m.field, formatTraceValue(record)))
		return false
	}
	return traceWrapped(value, buf, prefix, isLast, isRoot, fmt.Sprintf("Field(%q)", m.field), m.literal, false)
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

// buildArrayRecord applies a literal condition to an array value, inheriting
// compile-time extensions (without limits) so converters, getters, and custom
// operators work inside array fields.
func buildArrayRecord(condition any, ctx *compileCtx) (matcher, error) {
	conv := ctx.convOrNil()
	var children []matcher
	if _, ok := condition.([]any); ok {
		if conv != nil {
			children = append(children, eqConvMatcher{condition: condition, conv: conv})
		} else {
			children = append(children, eqMatcher{condition: condition})
		}
	}
	var (
		child matcher
		err   error
	)
	switch c := condition.(type) {
	case map[string]any:
		child, err = buildHashCondition(ctx, splitArrayCondition(ctx, c))
	case *regexp.Regexp:
		child, err = buildElemMatch(ctx, map[string]any{"$regex": c})
	default:
		child, err = buildElemMatch(ctx, map[string]any{"$eq": c})
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
func splitArrayCondition(ctx *compileCtx, condition map[string]any) map[string]any {
	parsed := map[string]any{}
	elem := map[string]any{}
	for _, key := range sortedKeys(condition) {
		value := condition[key]
		_, isIndex := parseIndexKey(key)
		var (
			customOp  Operator
			isCustom  bool
			isBuiltin bool
		)
		if ctx != nil {
			customOp, isCustom = ctx.customOps[key]
		}
		_, isBuiltin = operators[key]
		switch {
		case isIndex:
			parsed[key] = value
		case key == "$elemMatch":
			if m, ok := value.(map[string]any); ok {
				maps.Copy(elem, m)
			} else {
				parsed[key] = value
			}
		case isCustom:
			if customOp.MatchElements {
				elem[key] = value
			} else {
				parsed[key] = value
			}
		case isBuiltin:
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
