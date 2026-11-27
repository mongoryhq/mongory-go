package engine

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
)

// extLiteralMatcher handles literal values when compile-time extensions
// (converters, getters, or custom operators) are active, keeping the default
// literalMatcher zero-overhead.
type extLiteralMatcher struct {
	priorityNode
	condition   any
	dispatched  matcher
	arrayCtx    *compileCtx // nil limits; keeps converters, getters, customOps
	hasCustomOp bool
	hasConv     bool

	arrayOnce   sync.Once
	arrayReady  atomic.Bool
	arrayRecord matcher
}

func (m *extLiteralMatcher) ensureArrayRecord() {
	m.arrayOnce.Do(func() {
		if ar, err := buildArrayRecord(m.condition, m.arrayCtx); err == nil && ar != nil {
			publishArrayRecord(&m.priorityNode, ar, &m.arrayRecord, &m.arrayReady)
		}
	})
}

func (m *extLiteralMatcher) activeMatcher() matcher {
	if m.arrayReady.Load() {
		return m.arrayRecord
	}
	return m.dispatched
}

func (m *extLiteralMatcher) match(record any) bool {
	if _, ok := record.([]any); ok {
		m.ensureArrayRecord()
		if m.arrayRecord == nil {
			return false
		}
		if m.hasCustomOp {
			return evalMatcherOnRaw(m.arrayRecord, record)
		}
		return m.arrayRecord.match(record)
	}
	if m.hasCustomOp {
		return evalMatcherOnRaw(m.dispatched, record)
	}
	return m.dispatched.match(record)
}

func (m *extLiteralMatcher) matchReflect(rv reflect.Value) bool {
	if unwrapped, ok := unwrapReflect(rv); ok {
		if !m.hasCustomOp && unwrapped.Kind() == reflect.String && unwrapped.Type() == jsonNumberType {
			return m.match(parseJSONNumberString(unwrapped.String()))
		}
		if isReflectList(unwrapped) {
			m.ensureArrayRecord()
			return m.arrayRecord != nil && m.arrayRecord.matchReflect(rv)
		}
	}
	// Preserve outer pointer/typed-nil rv for custom operators and *T converters.
	return m.dispatched.matchReflect(rv)
}

func (m *extLiteralMatcher) priority() int { return m.activeMatcher().priority() }
func (m *extLiteralMatcher) explainNode() ExplainNode {
	return m.activeMatcher().explainNode()
}
func (m *extLiteralMatcher) traceNode(record any) (bool, TraceNode) {
	if isArrayRecordValue(record) {
		m.ensureArrayRecord()
		if m.arrayRecord == nil {
			return false, TraceNode{
				Kind:      "array_record",
				Status:    TraceFail,
				Label:     "ArrayRecord",
				Priority:  m.priority(),
				ValueText: formatTraceValue(record),
				Value:     snapshotTraceValue(record),
				Exists:    boolPtr(recordExists(record)),
			}
		}
		return m.arrayRecord.traceNode(record)
	}
	return m.dispatched.traceNode(record)
}

// extFieldMatcher handles field lookup when compile-time getters/resolvers or
// raw custom-operator values are active, leaving the default fieldMatcher free
// of extra checks.
type extFieldMatcher struct {
	*fieldMatcher
	getters *getterRegistry
	raw     bool
}

func (m *extFieldMatcher) evalLiteral(val any) bool {
	if m.raw {
		return evalMatcherOnRaw(m.literal, val)
	}
	return matchRaw(m.literal, val)
}

func (m *extFieldMatcher) match(record any) bool {
	value, ok := m.extract(record)
	if !ok {
		return false
	}
	return m.evalLiteral(value)
}

func (m *extFieldMatcher) matchReflect(rv reflect.Value) bool {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return false
	}
	var slot *fieldSlot
	var info *structInfo
	if m.getters != nil {
		slot, info = m.resolveExtSlot(rv.Type())
		if slot.getter != nil {
			return m.evalLiteral(slot.getter.invoke(rv, slot.getter.needPtr))
		}
		for _, r := range slot.resolvers {
			if val, ok := r.invoke(rv, r.needPtr, m.field); ok {
				return m.evalLiteral(val)
			}
		}
	}
	if rv.Kind() == reflect.Struct {
		return m.matchStructFieldExt(rv, slot, info)
	}
	if m.raw && (rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array) {
		if rv.Kind() == reflect.Slice && isKeyValueStruct(rv.Type().Elem()) {
			for i := range rv.Len() {
				entry := rv.Index(i)
				if entry.Field(0).String() == m.field {
					if v, ok := elemInterface(entry.Field(1)); ok {
						return m.evalLiteral(v)
					}
					return false
				}
			}
			return evalMatcherOnRaw(m.literal, missing)
		}
		if rv.Kind() == reflect.Array && rv.Type().Elem().Kind() == reflect.Uint8 {
			return false
		}
		if m.isIndex {
			i := m.index
			if i < 0 {
				i += rv.Len()
			}
			if i >= 0 && i < rv.Len() {
				return matchReflectValueRaw(m.literal, rv.Index(i), true)
			}
		}
		return evalMatcherOnRaw(m.literal, missing)
	}
	return m.fieldMatcher.matchReflect(rv)
}

func (m *extFieldMatcher) matchStructFieldExt(rv reflect.Value, slot *fieldSlot, info *structInfo) bool {
	if slot == nil {
		slot, info = m.resolveExtSlot(rv.Type())
	}
	if slot.isScalar {
		return false
	}
	if slot.isRegex {
		return m.match(normalizeReflect(rv))
	}
	if slot.found {
		if slot.field.singleIndex >= 0 {
			return matchReflectValueRaw(m.literal, rv.Field(slot.field.singleIndex), m.raw)
		}
		if fv, exists := fieldValue(rv, slot.field); exists {
			return matchReflectValueRaw(m.literal, fv, m.raw)
		}
	}
	if slot.hasInlineMaps {
		if info == nil {
			info = getStructInfo(rv.Type())
		}
		for _, im := range info.inlineMaps {
			if mv, exists := fieldValue(rv, im); exists && !mv.IsNil() {
				if elem := mv.MapIndex(m.fieldVal); elem.IsValid() {
					return matchReflectValueRaw(m.literal, elem, m.raw)
				}
			}
		}
	}
	return m.evalLiteral(missing)
}

func (m *extFieldMatcher) resolveExtSlot(t reflect.Type) (*fieldSlot, *structInfo) {
	if slot := m.slot.Load(); slot != nil && slot.typ == t {
		return slot, nil
	}
	slot, info := m.buildSlot(t, m.getters)
	if m.slot.Load() == nil {
		m.slot.CompareAndSwap(nil, slot)
	}
	return slot, info
}

func (m *extFieldMatcher) traceNode(record any) (bool, TraceNode) {
	value, ok := m.extractTraceValueWithGetters(record, m.getters)
	if !ok {
		return false, TraceNode{
			Kind:      "field",
			Status:    TraceFail,
			Label:     fmt.Sprintf("Field(%q)", m.field),
			Priority:  m.priority(),
			Field:     m.field,
			ValueText: formatTraceValue(record),
			Value:     snapshotTraceValue(record),
			Exists:    boolPtr(recordExists(record)),
		}
	}
	return traceWrappedNode(value, "field", "", m.field, fmt.Sprintf("Field(%q)", m.field), m.priority(), m.literal, false)
}

func matchReflectValueRaw(m matcher, rv reflect.Value, raw bool) bool {
	if rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return m.match(nil)
		}
		if raw {
			return evalMatcherOnRaw(m, rv.Interface())
		}
		return matchRaw(m, rv.Interface())
	}
	return m.matchReflect(rv)
}
