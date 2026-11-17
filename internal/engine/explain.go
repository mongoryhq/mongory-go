package engine

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

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

func statusLabel(matched bool) string {
	if matched {
		return "PASS"
	}
	return "FAIL"
}

func traceLeaf(m matcher, record any, buf *strings.Builder, prefix string, isLast, isRoot bool, label string) bool {
	matched := m.match(record)
	line := fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), label, formatValue(record))
	writeTreeLine(buf, prefix, isLast, isRoot, line)
	return matched
}

func explainBranch(buf *strings.Builder, prefix string, isLast, isRoot bool, label string, children ...matcher) {
	childPrefix := writeTreeLine(buf, prefix, isLast, isRoot, label)
	for i, c := range children {
		c.explain(buf, childPrefix, i == len(children)-1, false)
	}
}

func traceGroup(record any, buf *strings.Builder, prefix string, isLast, isRoot bool, op string, children []matcher, stopOn bool) bool {
	if len(children) == 0 {
		matched := !stopOn // empty And matches (stopOn=false -> true); empty Or never matches (stopOn=true -> false)
		writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[%s] %s (empty) | value=%s", statusLabel(matched), op, formatValue(record)))
		return matched
	}
	childPrefix := childPrefixFor(prefix, isLast, isRoot)
	var childBuf strings.Builder
	matched := !stopOn
	for i, c := range children {
		res := c.trace(record, &childBuf, childPrefix, i == len(children)-1, false)
		if res == stopOn {
			matched = stopOn
			if rem := len(children) - i - 1; rem > 0 {
				writeTreeLine(&childBuf, childPrefix, true, false, fmt.Sprintf("[SKIP] %d remaining branch(es) (short-circuited)", rem))
			}
			break
		}
	}
	writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), op, formatValue(record)))
	buf.WriteString(childBuf.String())
	return matched
}

func traceWrapped(record any, buf *strings.Builder, prefix string, isLast, isRoot bool, label string, inner matcher, invert bool) bool {
	childPrefix := childPrefixFor(prefix, isLast, isRoot)
	var childBuf strings.Builder
	matched := inner.trace(record, &childBuf, childPrefix, true, false)
	if invert {
		matched = !matched
	}
	writeTreeLine(buf, prefix, isLast, isRoot, fmt.Sprintf("[%s] %s | value=%s", statusLabel(matched), label, formatValue(record)))
	buf.WriteString(childBuf.String())
	return matched
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
	if s, ok := x.(fmt.Stringer); ok {
		return truncateString(s.String(), 64)
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
