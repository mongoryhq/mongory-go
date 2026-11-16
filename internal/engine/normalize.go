package engine

import (
	"encoding/json"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// missingKey marks a field absent from its parent, distinct from an explicit nil.
type missingKey struct{}

var missing any = missingKey{}

// timeValuer matches BSON datetime types such as primitive.DateTime without
// importing the driver.
type timeValuer interface {
	Time() time.Time
}

// normalize shallowly collapses a Go value into one of the canonical matching
// types: nil, bool, string, int64, float64, time.Time, map[string]any, []any,
// or an opaque comparable value ([N]byte identifiers such as ObjectID).
// Children of maps and slices stay raw; every reader normalizes on access, so
// structs and typed containers are only converted at the levels a query visits.
func normalize(v any) any {
	switch x := v.(type) {
	case nil, missingKey, bool, string, int64, float64, time.Time,
		map[string]any, []any, *regexp.Regexp:
		return v
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case uint32:
		return int64(x)
	case float32:
		return float64(x)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return string(x)
	}
	return normalizeReflect(reflect.ValueOf(v))
}

func normalizeReflect(rv reflect.Value) any {
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return normalize(rv.Elem().Interface())
	}
	if t, ok := rv.Interface().(timeValuer); ok {
		return t.Time()
	}
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.String:
		return rv.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := rv.Uint(); u <= math.MaxInt64 {
			return int64(u)
		}
		return float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		if isKeyValueStruct(rv.Type().Elem()) {
			return orderedDocToMap(rv)
		}
		return sliceToAny(rv)
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return rv.Interface() // identifier (ObjectID, UUID), not a list
		}
		return sliceToAny(rv)
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		if rv.Type().Key().Kind() != reflect.String {
			return rv.Interface()
		}
		out := make(map[string]any, rv.Len())
		for it := rv.MapRange(); it.Next(); {
			out[it.Key().String()] = it.Value().Interface()
		}
		return out
	case reflect.Struct:
		if isRegexStruct(rv.Type()) {
			return map[string]any{"$regex": rv.Field(0).String(), "$options": rv.Field(1).String()}
		}
		return structToMap(rv)
	}
	return rv.Interface()
}

// isRegexStruct recognizes BSON regex values such as primitive.Regex:
// a struct of exactly {Pattern string; Options string}. They become a
// {"$regex", "$options"} condition.
func isRegexStruct(t reflect.Type) bool {
	return t.NumField() == 2 &&
		t.Field(0).Name == "Pattern" && t.Field(0).Type.Kind() == reflect.String &&
		t.Field(1).Name == "Options" && t.Field(1).Type.Kind() == reflect.String
}

func sliceToAny(rv reflect.Value) []any {
	out := make([]any, rv.Len())
	for i := range out {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

// isKeyValueStruct recognizes ordered-document elements such as bson.E:
// a struct of exactly {Key string; Value any}.
func isKeyValueStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.NumField() == 2 &&
		t.Field(0).Name == "Key" && t.Field(0).Type.Kind() == reflect.String &&
		t.Field(1).Name == "Value" && t.Field(1).Type.Kind() == reflect.Interface
}

// orderedDocToMap converts a bson.D-like slice into a document.
func orderedDocToMap(rv reflect.Value) map[string]any {
	out := make(map[string]any, rv.Len())
	for i := range rv.Len() {
		e := rv.Index(i)
		out[e.Field(0).String()] = e.Field(1).Interface()
	}
	return out
}

type structField struct {
	name      string
	index     []int
	inlineMap bool
}

var structFieldCache sync.Map // reflect.Type -> []structField

func structToMap(rv reflect.Value) map[string]any {
	fields := structFields(rv.Type())
	out := make(map[string]any, len(fields))
	var inlineMaps []reflect.Value
	for _, f := range fields {
		fv, err := rv.FieldByIndexErr(f.index)
		if err != nil {
			continue // field promoted through a nil inline pointer
		}
		if f.inlineMap {
			inlineMaps = append(inlineMaps, fv)
			continue
		}
		out[f.name] = fv.Interface()
	}
	// Inline map entries never override declared fields.
	for _, m := range inlineMaps {
		for it := m.MapRange(); it.Next(); {
			if k := it.Key().String(); !hasKey(out, k) {
				out[k] = it.Value().Interface()
			}
		}
	}
	return out
}

func hasKey(m map[string]any, k string) bool {
	_, ok := m[k]
	return ok
}

func structFields(t reflect.Type) []structField {
	if cached, ok := structFieldCache.Load(t); ok {
		return cached.([]structField)
	}
	var fields []structField
	collectStructFields(t, nil, map[string]bool{}, &fields)
	cached, _ := structFieldCache.LoadOrStore(t, fields)
	return cached.([]structField)
}

// collectStructFields follows the mongo-go-driver struct codec: exported
// fields only, key from the tag or the lowercased field name, and only fields
// tagged ",inline" are flattened. Direct fields are added before inlined
// ones, so a shallower field shadows a deeper one with the same key.
func collectStructFields(t reflect.Type, prefix []int, seen map[string]bool, out *[]structField) {
	var inlined []reflect.StructField
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name, inline := structFieldKey(sf)
		if name == "-" {
			continue
		}
		index := append(slices.Clone(prefix), i)
		if inline {
			switch ft := derefType(sf.Type); {
			case ft.Kind() == reflect.Struct:
				inlined = append(inlined, sf)
				continue
			case ft.Kind() == reflect.Map && ft.Key().Kind() == reflect.String:
				*out = append(*out, structField{index: index, inlineMap: true})
				continue
			}
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		*out = append(*out, structField{name: name, index: index})
	}
	for _, sf := range inlined {
		collectStructFields(derefType(sf.Type), append(slices.Clone(prefix), sf.Index...), seen, out)
	}
}

func derefType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}

// structFieldKey reads the mongory tag, else the bson tag, else falls back to
// the lowercased field name, as the mongo-go-driver does. Like the driver, a
// bare tag without any "key:" pair (`"name,omitempty"`) counts as a bson tag.
func structFieldKey(sf reflect.StructField) (name string, inline bool) {
	tag, ok := sf.Tag.Lookup("mongory")
	if !ok {
		tag, ok = sf.Tag.Lookup("bson")
	}
	if !ok && !strings.Contains(string(sf.Tag), ":") {
		tag = string(sf.Tag)
	}
	name, opts, _ := strings.Cut(tag, ",")
	inline = hasInlineOption(opts)
	if name == "" {
		name = strings.ToLower(sf.Name)
	}
	return name, inline
}

func hasInlineOption(opts string) bool {
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == "inline" {
			return true
		}
	}
	return false
}

// convertCondition expands dotted keys into nested conditions (deep-merging
// shared prefixes) and normalizes every value. Keys are visited in sorted
// order so overlapping keys merge deterministically.
func convertCondition(condition map[string]any) map[string]any {
	out := make(map[string]any, len(condition))
	for _, key := range sortedKeys(condition) {
		deepMerge(out, expandKey(key, convertValue(condition[key])))
	}
	return out
}

func convertValue(v any) any {
	switch x := normalize(v).(type) {
	case map[string]any:
		return convertCondition(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = convertValue(e)
		}
		return out
	default:
		return x
	}
}

// expandKey turns "a.b\.c" into {"a": {"b.c": value}}.
func expandKey(key string, value any) map[string]any {
	parts := splitKey(key)
	result := map[string]any{parts[len(parts)-1]: value}
	for i := len(parts) - 2; i >= 0; i-- {
		result = map[string]any{parts[i]: result}
	}
	return result
}

// splitKey splits on unescaped dots and unescapes "\.".
func splitKey(key string) []string {
	var parts []string
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		switch {
		case key[i] == '\\' && i+1 < len(key) && key[i+1] == '.':
			b.WriteByte('.')
			i++
		case key[i] == '.':
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteByte(key[i])
		}
	}
	return append(parts, b.String())
}

func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		dm, dok := dst[k].(map[string]any)
		sm, sok := sv.(map[string]any)
		if dok && sok {
			deepMerge(dm, sm)
			continue
		}
		dst[k] = sv
	}
}
