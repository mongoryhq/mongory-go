package engine

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// TraceStatus is the evaluation outcome of one node in a TraceNode tree.
type TraceStatus string

const (
	TracePass TraceStatus = "pass"
	TraceFail TraceStatus = "fail"
	TraceSkip TraceStatus = "skip"
)

// ExplainNode is one node in the structured tree returned by ExplainTree.
type ExplainNode struct {
	Kind     string        `json:"kind"`
	Label    string        `json:"label"`
	Priority int           `json:"priority"`
	Operator string        `json:"operator,omitempty"`
	Field    string        `json:"field,omitempty"`
	Children []ExplainNode `json:"children,omitempty"`
}

// TraceNode is one node in the structured evaluation trace returned by TraceTree.
type TraceNode struct {
	Kind         string      `json:"kind"`
	Status       TraceStatus `json:"status"`
	Label        string      `json:"label"`
	Priority     int         `json:"priority"`
	Operator     string      `json:"operator,omitempty"`
	Field        string      `json:"field,omitempty"`
	Index        *int        `json:"index,omitempty"`
	SkippedCount int         `json:"skipped_count,omitempty"`
	ValueText    string      `json:"value_text,omitempty"`
	Value        any         `json:"value,omitempty"`
	Exists       *bool       `json:"exists,omitempty"`
	Children     []TraceNode `json:"children,omitempty"`
}

func traceStatus(matched bool) TraceStatus {
	if matched {
		return TracePass
	}
	return TraceFail
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func recordExists(record any) bool {
	_, isMissing := record.(missingKey)
	return !isMissing
}

func renderExplainNode(buf *strings.Builder, node *ExplainNode, prefix string, isLast, isRoot bool) {
	childPrefix := writeTreeLine(buf, prefix, isLast, isRoot, node.Label)
	for i := range node.Children {
		renderExplainNode(buf, &node.Children[i], childPrefix, i == len(node.Children)-1, false)
	}
}

func renderTraceNode(buf *strings.Builder, node *TraceNode, prefix string, isLast, isRoot bool) {
	var line string
	if node.Kind == "skip" {
		line = fmt.Sprintf("[SKIP] %s", node.Label)
	} else {
		line = fmt.Sprintf("[%s] %s | value=%s", strings.ToUpper(string(node.Status)), node.Label, node.ValueText)
	}
	childPrefix := writeTreeLine(buf, prefix, isLast, isRoot, line)
	for i := range node.Children {
		renderTraceNode(buf, &node.Children[i], childPrefix, i == len(node.Children)-1, false)
	}
}

func snapshotTraceValue(v any) any {
	return snapshotTraceValueDepth(v, nil, 0)
}

const maxTraceSnapshotDepth = 8

func snapshotTraceValueDepth(v any, seen map[cycleKey]bool, depth int) any {
	if v == nil {
		return nil
	}
	if _, isMissing := v.(missingKey); isMissing {
		return nil
	}
	if depth >= maxTraceSnapshotDepth {
		return formatTraceValue(v)
	}
	switch x := v.(type) {
	case bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return x
	case float32:
		f := float64(x)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return strconv.FormatFloat(f, 'g', -1, 32)
		}
		return f
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return strconv.FormatFloat(x, 'g', -1, 64)
		}
		return x
	case time.Time:
		return x
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = snapshotTraceValueDepth(item, seen, depth+1)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = snapshotTraceValueDepth(val, seen, depth+1)
		}
		return out
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil
		}
		nextSeen, cyclic := enterTraceCycle(seen, cycleKey{typ: rv.Type(), ptr: rv.Pointer()})
		if cyclic {
			return "[cyclic]"
		}
		return snapshotTraceValueDepth(rv.Elem().Interface(), nextSeen, depth+1)
	}
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.String:
		return rv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint()
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
		return f
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		nextSeen, cyclic := enterTraceCycle(seen, cycleKey{typ: rv.Type(), ptr: rv.Pointer()})
		if cyclic {
			return "[cyclic]"
		}
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			elem := rv.Index(i)
			if elem.CanInterface() {
				out[i] = snapshotTraceValueDepth(elem.Interface(), nextSeen, depth+1)
			}
		}
		return out
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return formatTraceValue(v)
		}
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			elem := rv.Index(i)
			if elem.CanInterface() {
				out[i] = snapshotTraceValueDepth(elem.Interface(), seen, depth+1)
			}
		}
		return out
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		nextSeen, cyclic := enterTraceCycle(seen, cycleKey{typ: rv.Type(), ptr: rv.Pointer()})
		if cyclic {
			return "[cyclic]"
		}
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			k := fmt.Sprint(iter.Key().Interface())
			val := iter.Value()
			if val.CanInterface() {
				out[k] = snapshotTraceValueDepth(val.Interface(), nextSeen, depth+1)
			}
		}
		return out
	case reflect.Struct:
		if t, ok := extractTime(rv); ok {
			return t
		}
		return formatTraceValue(v)
	default:
		return formatTraceValue(v)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func childPrefixFor(prefix string, isLast, isRoot bool) string {
	if isRoot {
		return ""
	}
	if isLast {
		return prefix + "    "
	}
	return prefix + "│   "
}

func writeTreeLine(buf *strings.Builder, prefix string, isLast, isRoot bool, line string) string {
	if isRoot {
		buf.WriteString(line)
		buf.WriteByte('\n')
		return ""
	}
	buf.WriteString(prefix)
	if isLast {
		buf.WriteString("└── ")
	} else {
		buf.WriteString("├── ")
	}
	buf.WriteString(line)
	buf.WriteByte('\n')
	return childPrefixFor(prefix, isLast, false)
}

func traceLeafNode(m matcher, record any, kind, operator, label string) (bool, TraceNode) {
	matched := matchRaw(m, record)
	return matched, TraceNode{
		Kind:      kind,
		Status:    traceStatus(matched),
		Label:     label,
		Priority:  m.priority(),
		Operator:  operator,
		ValueText: formatTraceValue(record),
		Value:     snapshotTraceValue(record),
		Exists:    boolPtr(recordExists(record)),
	}
}

func explainBranchNode(kind, operator, field, label string, priority int, children ...matcher) ExplainNode {
	out := ExplainNode{
		Kind:     kind,
		Label:    label,
		Priority: priority,
		Operator: operator,
		Field:    field,
	}
	if len(children) > 0 {
		out.Children = make([]ExplainNode, len(children))
		for i, c := range children {
			out.Children[i] = c.explainNode()
		}
	}
	return out
}

func traceGroupNode(record any, kind, operator, op string, priority int, children []matcher, stopOn bool) (bool, TraceNode) {
	if len(children) == 0 {
		matched := !stopOn // empty And matches (stopOn=false -> true); empty Or never matches (stopOn=true -> false)
		return matched, TraceNode{
			Kind:      kind,
			Status:    traceStatus(matched),
			Label:     op + " (empty)",
			Priority:  priority,
			Operator:  operator,
			ValueText: formatTraceValue(record),
			Value:     snapshotTraceValue(record),
			Exists:    boolPtr(recordExists(record)),
		}
	}
	childNodes := make([]TraceNode, 0, len(children))
	matched := !stopOn
	for i, c := range children {
		res, child := c.traceNode(record)
		childNodes = append(childNodes, child)
		if res == stopOn {
			matched = stopOn
			if rem := len(children) - i - 1; rem > 0 {
				childNodes = append(childNodes, TraceNode{
					Kind:         "skip",
					Status:       TraceSkip,
					Label:        fmt.Sprintf("%d remaining branch(es) (short-circuited)", rem),
					SkippedCount: rem,
				})
			}
			break
		}
	}
	return matched, TraceNode{
		Kind:      kind,
		Status:    traceStatus(matched),
		Label:     op,
		Priority:  priority,
		Operator:  operator,
		ValueText: formatTraceValue(record),
		Value:     snapshotTraceValue(record),
		Exists:    boolPtr(recordExists(record)),
		Children:  childNodes,
	}
}

func traceWrappedNode(record any, kind, operator, field, label string, priority int, inner matcher, invert bool) (bool, TraceNode) {
	matched, child := inner.traceNode(record)
	if invert {
		matched = !matched
	}
	return matched, TraceNode{
		Kind:      kind,
		Status:    traceStatus(matched),
		Label:     label,
		Priority:  priority,
		Operator:  operator,
		Field:     field,
		ValueText: formatTraceValue(record),
		Value:     snapshotTraceValue(record),
		Exists:    boolPtr(recordExists(record)),
		Children:  []TraceNode{child},
	}
}

func safeStringer(v any) (string, bool) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return "nil", true
	}
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return "nil", true
	}
	if s, ok := v.(fmt.Stringer); ok {
		return truncateString(s.String(), 64), true
	}
	return "", false
}

func formatTraceValue(v any) string {
	switch x := v.(type) {
	case missingKey:
		return "<missing>"
	case nil:
		return "nil"
	case string, bool, int, int64, float64, time.Time, []any, map[string]any:
		return formatValueSeen(x, nil)
	}
	if s, ok := safeStringer(v); ok {
		return s
	}
	if norm, ok := normalizeFast(v); ok {
		return formatValueSeen(norm, nil)
	}
	return formatValueSeen(normalize(v), nil)
}

func formatValue(v any) string {
	return formatValueSeen(v, nil)
}

func formatValueSeen(v any, seen map[cycleKey]bool) string {
	switch x := v.(type) {
	case missingKey:
		return "<missing>"
	case nil:
		return "nil"
	case string:
		return truncateString(strconv.Quote(x), 64)
	case bool, int, int64, float64:
		return fmt.Sprint(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case []any:
		return formatSliceElems(len(x), reflect.ValueOf(x).Pointer(), func(i int) any { return x[i] }, seen)
	case map[string]any:
		return formatStringMap(x, reflect.ValueOf(x).Pointer(), seen)
	default:
		return formatFallbackValue(x, seen)
	}
}

func enterTraceCycle(seen map[cycleKey]bool, k cycleKey) (map[cycleKey]bool, bool) {
	if seen[k] {
		return seen, false
	}
	if seen == nil {
		seen = map[cycleKey]bool{}
	}
	seen[k] = true
	return seen, true
}

func formatSliceElems(n int, ptr uintptr, elem func(int) any, seen map[cycleKey]bool) string {
	if n > 0 {
		k := cycleKey{kind: reflect.Slice, ptr: ptr, n: n}
		var ok bool
		if seen, ok = enterTraceCycle(seen, k); !ok {
			return "<cycle>"
		}
		defer delete(seen, k)
	}
	var b strings.Builder
	b.WriteByte('[')
	limit := min(n, 4)
	for i := range limit {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(formatValueSeen(elem(i), seen))
	}
	if n > limit {
		fmt.Fprintf(&b, ", ...(+%d)", n-limit)
	}
	b.WriteByte(']')
	return b.String()
}

func formatStringMap(m map[string]any, ptr uintptr, seen map[cycleKey]bool) string {
	if len(m) > 0 {
		k := cycleKey{kind: reflect.Map, ptr: ptr}
		var ok bool
		if seen, ok = enterTraceCycle(seen, k); !ok {
			return "<cycle>"
		}
		defer delete(seen, k)
	}
	var b strings.Builder
	b.WriteByte('{')
	keys := sortedKeys(m)
	limit := min(len(keys), 4)
	for i := range limit {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s: %s", strconv.Quote(keys[i]), formatValueSeen(m[keys[i]], seen))
	}
	if len(keys) > limit {
		fmt.Fprintf(&b, ", ...(+%d)", len(keys)-limit)
	}
	b.WriteByte('}')
	return b.String()
}

func formatFallbackValue(x any, seen map[cycleKey]bool) string {
	if s, ok := safeStringer(x); ok {
		return s
	}
	rv := reflect.ValueOf(x)
	if rv.IsValid() {
		switch rv.Kind() {
		case reflect.Slice:
			return formatSliceElems(rv.Len(), rv.Pointer(), func(i int) any { return rv.Index(i).Interface() }, seen)
		case reflect.Map:
			if rv.IsNil() {
				return "nil"
			}
			if rv.Type().Key().Kind() == reflect.String {
				return formatStringMap(reflectMapToAny(rv), rv.Pointer(), seen)
			}
			if rv.Len() > 0 {
				k := cycleKey{kind: reflect.Map, ptr: rv.Pointer()}
				var ok bool
				if seen, ok = enterTraceCycle(seen, k); !ok {
					return "<cycle>"
				}
				defer delete(seen, k)
			}
			var b strings.Builder
			b.WriteByte('{')
			i := 0
			for it := rv.MapRange(); it.Next() && i < 4; i++ {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s: %s", formatValueSeen(it.Key().Interface(), seen), formatValueSeen(it.Value().Interface(), seen))
			}
			if rv.Len() > 4 {
				fmt.Fprintf(&b, ", ...(+%d)", rv.Len()-4)
			}
			b.WriteByte('}')
			return b.String()
		}
	}
	return truncateString(fmt.Sprintf("%v", x), 64)
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
