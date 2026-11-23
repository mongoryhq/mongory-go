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

var (
	timeType        = reflect.TypeOf(time.Time{})
	timeValuerType  = reflect.TypeOf((*timeValuer)(nil)).Elem()
	jsonNumberType  = reflect.TypeOf(json.Number(""))
	regexpType      = reflect.TypeOf((*regexp.Regexp)(nil))
	structInfoCache sync.Map // reflect.Type -> *structInfo
	timeValuerCache sync.Map // reflect.Type -> bool
)

// normalize shallowly collapses a Go value into one of the canonical matching
// types: nil, bool, string, int64, float64, time.Time, map[string]any, []any,
// or an opaque comparable value ([N]byte identifiers such as ObjectID).
// Children of maps and slices stay raw; every reader normalizes on access, so
// structs and typed containers are only converted at the levels a query visits.
func normalize(v any) any {
	if norm, ok := normalizeFast(v); ok {
		return norm
	}
	return normalizeReflect(reflect.ValueOf(v))
}

// normalizeFast collapses canonical values, primitive integers/floats, common
// scalar pointers, and json.Number without reflection. It returns ok=false for
// aggregate or custom reflect types (structs, typed slices/maps, interfaces).
func normalizeFast(v any) (any, bool) {
	switch x := v.(type) {
	case nil, missingKey, bool, string, int64, float64, time.Time,
		map[string]any, []any:
		return v, true
	case *regexp.Regexp:
		if x == nil {
			return nil, true
		}
		return x, true
	case int:
		return int64(x), true
	case int32:
		return int64(x), true
	case uint32:
		return int64(x), true
	case float32:
		return float64(x), true
	}
	return normalizeFastExtra(v)
}

func normalizeFastExtra(v any) (any, bool) {
	switch x := v.(type) {
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case *int:
		if x == nil {
			return nil, true
		}
		return int64(*x), true
	case *int64:
		if x == nil {
			return nil, true
		}
		return *x, true
	case *float64:
		if x == nil {
			return nil, true
		}
		return *x, true
	case *string:
		if x == nil {
			return nil, true
		}
		return *x, true
	case *bool:
		if x == nil {
			return nil, true
		}
		return *x, true
	case *time.Time:
		if x == nil {
			return nil, true
		}
		return *x, true
	case json.Number:
		return parseJSONNumber(x), true
	}
	return nil, false
}

func parseJSONNumber(x json.Number) any {
	if i, err := x.Int64(); err == nil {
		return i
	}
	if f, err := x.Float64(); err == nil {
		return f
	}
	return string(x)
}

func parseJSONNumberString(s string) any {
	return parseJSONNumber(json.Number(s))
}

func hasTimeValuer(t reflect.Type) bool {
	if t.PkgPath() == "" {
		return false
	}
	if cached, ok := timeValuerCache.Load(t); ok {
		return cached.(bool)
	}
	impl := t.Implements(timeValuerType)
	timeValuerCache.Store(t, impl)
	return impl
}

// extractTime returns the time.Time if rv is time.Time or implements timeValuer.
func extractTime(rv reflect.Value) (time.Time, bool) {
	t := rv.Type()
	if t == timeType {
		return rv.Interface().(time.Time), true
	}
	if hasTimeValuer(t) {
		return rv.Interface().(timeValuer).Time(), true
	}
	return time.Time{}, false
}

const maxPointerDepth = 64

// unwrapReflect dereferences pointers and interfaces and maps nil containers
// (slice, map, pointer, interface) to ok=false (representing nil).
// *regexp.Regexp is preserved intact, and pointer cycles stop at maxPointerDepth.
func unwrapReflect(rv reflect.Value) (reflect.Value, bool) {
	depth := 0
	for rv.IsValid() {
		switch rv.Kind() {
		case reflect.Pointer, reflect.Interface:
			if rv.IsNil() {
				return reflect.Value{}, false
			}
			if rv.Kind() == reflect.Pointer && rv.Type() == regexpType {
				return rv, true
			}
			depth++
			if depth > maxPointerDepth {
				return reflect.Value{}, false
			}
			rv = rv.Elem()
		case reflect.Slice, reflect.Map:
			if rv.IsNil() {
				return reflect.Value{}, false
			}
			return rv, true
		default:
			return rv, true
		}
	}
	return reflect.Value{}, false
}

// isReflectList reports whether rv should follow the implicit/explicit array
// path. It excludes bson.D ([]bson.E) and identifier byte arrays ([N]byte).
func isReflectList(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Slice:
		return !rv.IsNil() && !isKeyValueStruct(rv.Type().Elem())
	case reflect.Array:
		return rv.Type().Elem().Kind() != reflect.Uint8
	}
	return false
}

func normalizeLeafReflect(rv reflect.Value) (any, bool) {
	if t, ok := extractTime(rv); ok {
		return t, true
	}
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool(), true
	case reflect.String:
		if rv.Type() == jsonNumberType {
			return parseJSONNumberString(rv.String()), true
		}
		return rv.String(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := rv.Uint(); u <= math.MaxInt64 {
			return int64(u), true
		}
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	case reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return rv.Interface(), true // identifier (ObjectID, UUID), not a list
		}
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return rv.Interface(), true
		}
	case reflect.Struct:
		if isRegexStruct(rv.Type()) {
			return map[string]any{"$regex": rv.Field(0).String(), "$options": rv.Field(1).String()}, true
		}
	}
	return nil, false
}

func reflectMapToAny(rv reflect.Value) map[string]any {
	out := make(map[string]any, rv.Len())
	for it := rv.MapRange(); it.Next(); {
		out[it.Key().String()] = it.Value().Interface()
	}
	return out
}

func normalizeReflect(rv reflect.Value) any {
	rv, ok := unwrapReflect(rv)
	if !ok {
		return nil
	}
	if v, handled := normalizeLeafReflect(rv); handled {
		return v
	}
	switch rv.Kind() {
	case reflect.Slice:
		if isKeyValueStruct(rv.Type().Elem()) {
			return orderedDocToMap(rv)
		}
		return sliceToAny(rv)
	case reflect.Array:
		return sliceToAny(rv)
	case reflect.Map:
		return reflectMapToAny(rv)
	case reflect.Struct:
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
	name        string
	index       []int
	singleIndex int // index[0] when len(index) == 1, else -1
	inlineMap   bool
}

type structInfo struct {
	fields       []structField
	byName       map[string]structField
	inlineMaps   []structField
	isTimeValuer bool
	isRegex      bool
}

func fieldValue(rv reflect.Value, f structField) (reflect.Value, bool) {
	if f.singleIndex >= 0 {
		return rv.Field(f.singleIndex), true
	}
	cur := rv
	for i, idx := range f.index {
		if i > 0 && cur.Kind() == reflect.Pointer {
			if cur.IsNil() {
				return reflect.Value{}, false
			}
			cur = cur.Elem()
		}
		cur = cur.Field(idx)
	}
	return cur, true
}

func structIsPresent(rv reflect.Value) bool {
	info := getStructInfo(rv.Type())
	for _, f := range info.fields {
		fv, ok := fieldValue(rv, f)
		if !ok {
			continue
		}
		if !f.inlineMap {
			return true
		}
		if fv.Len() > 0 {
			return true
		}
	}
	return false
}

func structToMap(rv reflect.Value) map[string]any {
	info := getStructInfo(rv.Type())
	out := make(map[string]any, len(info.fields))
	var inlineMaps []reflect.Value
	for _, f := range info.fields {
		fv, ok := fieldValue(rv, f)
		if !ok {
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

func getStructInfo(t reflect.Type) *structInfo {
	if cached, ok := structInfoCache.Load(t); ok {
		return cached.(*structInfo)
	}
	var fields []structField
	collectStructFields(t, nil, map[string]bool{}, map[reflect.Type]bool{}, &fields)
	byName := make(map[string]structField, len(fields))
	var inlineMaps []structField
	for i := range fields {
		if len(fields[i].index) == 1 {
			fields[i].singleIndex = fields[i].index[0]
		} else {
			fields[i].singleIndex = -1
		}
		if fields[i].inlineMap {
			inlineMaps = append(inlineMaps, fields[i])
		} else {
			byName[fields[i].name] = fields[i]
		}
	}
	info := &structInfo{
		fields:       fields,
		byName:       byName,
		inlineMaps:   inlineMaps,
		isTimeValuer: t == timeType || hasTimeValuer(t),
		isRegex:      isRegexStruct(t),
	}
	cached, _ := structInfoCache.LoadOrStore(t, info)
	return cached.(*structInfo)
}

// collectStructFields resolves exported struct fields only (including exported
// fields promoted through an unexported anonymous ",inline" struct): key
// precedence is mongory tag, bson tag, json tag, bare tag, or the exact Go field
// name (no lowercasing). "-" tags also fall back to the exact Go field name.
// Only fields tagged ",inline" are flattened; direct fields are added before
// inlined ones, so a shallower field shadows a deeper one. visiting guards
// recursive ",inline" pointer types against stack overflow.
func collectStructFields(t reflect.Type, prefix []int, seen map[string]bool, visiting map[reflect.Type]bool, out *[]structField) {
	if visiting[t] {
		return
	}
	visiting[t] = true
	defer delete(visiting, t)

	var inlined []reflect.StructField
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			if sf.Anonymous && derefType(sf.Type).Kind() == reflect.Struct {
				if _, inline := structFieldKey(sf); inline {
					inlined = append(inlined, sf)
				}
			}
			continue
		}
		name, inline := structFieldKey(sf)
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
		collectStructFields(derefType(sf.Type), append(slices.Clone(prefix), sf.Index...), seen, visiting, out)
	}
}

func derefType(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}

var structTagOrder = []string{"mongory", "bson", "json"}

// structFieldKey resolves a struct field key by checking mongory, bson, json,
// and bare (without "key:") tags in order; the first explicit name wins,
// otherwise it falls back to sf.Name without lowercasing. Empty names (such
// as ",omitempty") and "-" tags yield no name so resolution continues to the
// next tag or sf.Name (json:"-," keeps literal "-").
func structFieldKey(sf reflect.StructField) (name string, inline bool) {
	for _, key := range structTagOrder {
		tag, ok := sf.Tag.Lookup(key)
		if !ok {
			continue
		}
		n, tagInline := parseFieldTag(tag, key == "json")
		inline = inline || tagInline
		if n != "" {
			return n, inline
		}
	}
	if !strings.Contains(string(sf.Tag), ":") {
		n, tagInline := parseFieldTag(string(sf.Tag), false)
		inline = inline || tagInline
		if n != "" {
			return n, inline
		}
	}
	return sf.Name, inline
}

func parseFieldTag(tag string, isJSON bool) (name string, inline bool) {
	if isJSON && tag == "-" {
		return "", false
	}
	n, opts, _ := strings.Cut(tag, ",")
	inline = hasInlineOption(opts)
	if !isJSON && n == "-" {
		return "", inline
	}
	return n, inline
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

// cycleKey identifies an active container or pointer on the recursion stack so
// cyclic references are rejected without false-positiving on shared DAGs or
// overlapping non-cyclic subslices (distinguished by length n).
type cycleKey struct {
	kind reflect.Kind
	typ  reflect.Type
	ptr  uintptr
	n    int
}

type normOpts struct {
	maxDepth  int
	conv      *converterRegistry
	customOps map[string]Operator
}

// convertRootCondition normalizes a top-level query condition document,
// expands dotted keys (turning leaf collisions into $and clauses), coerces
// typed nil slices passed to list operators into empty []any, runs registered
// converters on built-in arguments while preserving raw custom-op arguments,
// enforces maxDepth, and rejects cyclic values with ErrInvalidCondition.
func convertRootCondition(condition any, cfg CompileConfig) (map[string]any, error) {
	if condition == nil {
		return map[string]any{}, nil
	}
	opts := normOpts{
		maxDepth:  cfg.MaxDepth,
		conv:      cfg.converters,
		customOps: cfg.operators,
	}
	if m, ok := condition.(map[string]any); ok {
		if len(m) == 0 {
			return map[string]any{}, nil
		}
		seen := map[cycleKey]bool{}
		res, err := convertConditionWithSeen(m, 0, opts, seen)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
	seen := map[cycleKey]bool{}
	norm, err := convertAny(condition, false, 0, opts, seen)
	if err != nil {
		return nil, err
	}
	switch c := norm.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return c, nil
	default:
		return nil, invalidCondition("condition must be a document, got %T", condition)
	}
}

func checkDepthLimit(depth, maxDepth int) error {
	if maxDepth > 0 && depth > maxDepth {
		return limitExceeded("condition depth %d exceeds max depth %d", depth, maxDepth)
	}
	return nil
}

// convertConditionWithSeen expands dotted keys into nested conditions
// (preserving leaf collisions inside "$and") and normalizes every value. Keys
// are visited in sorted order so overlapping keys merge deterministically.
func convertConditionWithSeen(condition map[string]any, depth int, opts normOpts, seen map[cycleKey]bool) (map[string]any, error) {
	out := make(map[string]any, len(condition))
	var conflicts []any
	for _, key := range sortedKeys(condition) {
		parts := splitKey(key)
		nextDepth := depth + len(parts)
		if err := checkDepthLimit(nextDepth, opts.maxDepth); err != nil {
			return nil, err
		}
		lastKey := parts[len(parts)-1]
		var val any
		if _, isCustom := opts.customOps[lastKey]; isCustom {
			val = condition[key]
		} else {
			listCtx := isListOperator(lastKey)
			var err error
			val, err = convertAny(condition[key], listCtx, nextDepth, opts, seen)
			if err != nil {
				return nil, err
			}
		}
		if res := mergeExpandedNode(out, expandParts(parts, val)); len(res) > 0 {
			conflicts = append(conflicts, res)
		}
	}
	if len(conflicts) > 0 {
		if existing, ok := out["$and"]; ok {
			if arr, isArr := existing.([]any); isArr {
				out["$and"] = append(arr, conflicts...)
			}
		} else {
			out["$and"] = conflicts
		}
	}
	return out, nil
}

func isListOperator(key string) bool {
	switch key {
	case "$in", "$nin", "$and", "$or":
		return true
	}
	return false
}

func convertAny(v any, listCtx bool, depth int, opts normOpts, seen map[cycleKey]bool) (any, error) {
	if opts.conv != nil && v != nil {
		if entry, ok := opts.conv.lookupConcrete(reflect.TypeOf(v)); ok && reflect.TypeOf(v).Kind() != reflect.Pointer {
			noConv := opts
			noConv.conv = nil
			return convertAny(entry.convertAny(v), listCtx, depth, noConv, seen)
		}
	}
	if norm, ok := normalizeFast(v); ok {
		switch x := norm.(type) {
		case map[string]any:
			if len(x) == 0 {
				return map[string]any{}, nil
			}
			k := cycleKey{kind: reflect.Map, ptr: reflect.ValueOf(x).Pointer()}
			if seen[k] {
				return nil, invalidCondition("condition contains a cyclic reference")
			}
			seen[k] = true
			out, err := convertConditionWithSeen(x, depth, opts, seen)
			delete(seen, k)
			return out, err
		case []any:
			if len(x) == 0 {
				return make([]any, 0), nil
			}
			k := cycleKey{kind: reflect.Slice, ptr: reflect.ValueOf(x).Pointer(), n: len(x)}
			if seen[k] {
				return nil, invalidCondition("condition contains a cyclic reference")
			}
			seen[k] = true
			out, err := convertSliceAny(x, depth, opts, seen)
			delete(seen, k)
			return out, err
		default:
			return x, nil
		}
	}
	return convertReflect(reflect.ValueOf(v), listCtx, depth, opts, seen)
}

func convertSliceAny(x []any, depth int, opts normOpts, seen map[cycleKey]bool) ([]any, error) {
	out := make([]any, len(x))
	for i, e := range x {
		v, err := convertListElem(e, depth, opts, seen)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func convertListElem(e any, depth int, opts normOpts, seen map[cycleKey]bool) (any, error) {
	// Direct list elements inherit depth since the enclosing key already counted +1;
	// deeper nested arrays/containers increment on entry so [[...]] bombs fail cleanly.
	nextDepth := depth
	if isNestedListElem(e) {
		nextDepth++
		if err := checkDepthLimit(nextDepth, opts.maxDepth); err != nil {
			return nil, err
		}
	}
	return convertAny(e, false, nextDepth, opts, seen)
}

func isNestedListElem(v any) bool {
	if v == nil {
		return false
	}
	if _, ok := v.([]any); ok {
		return true
	}
	rv := reflect.ValueOf(v)
	for depth := 0; depth < maxPointerDepth && rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface); depth++ {
		if rv.IsNil() || (rv.Kind() == reflect.Pointer && rv.Type() == regexpType) {
			return false
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return false
	}
	return (rv.Kind() == reflect.Slice && !isKeyValueStruct(rv.Type().Elem())) ||
		(rv.Kind() == reflect.Array && rv.Type().Elem().Kind() != reflect.Uint8)
}

func convertReflect(rv reflect.Value, listCtx bool, depth int, opts normOpts, seen map[cycleKey]bool) (any, error) {
	var pushedBuf [4]cycleKey
	pushed := pushedBuf[:0]
	defer func() {
		for _, k := range pushed {
			delete(seen, k)
		}
	}()

	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return nil, nil
		}
		if rv.Kind() == reflect.Pointer {
			t := rv.Type()
			if t == regexpType {
				return rv.Interface(), nil
			}
			k := cycleKey{kind: reflect.Pointer, typ: t, ptr: rv.Pointer()}
			if seen[k] {
				return nil, invalidCondition("condition contains a cyclic reference")
			}
			seen[k] = true
			pushed = append(pushed, k)
			if opts.conv != nil {
				if entry, ok := opts.conv.lookupConcrete(t); ok {
					noConv := opts
					noConv.conv = nil
					return convertAny(entry.convertReflect(rv), listCtx, depth, noConv, seen)
				}
				if entry, ok := opts.conv.lookupIface(t); ok {
					noConv := opts
					noConv.conv = nil
					return convertAny(entry.convertReflect(rv), listCtx, depth, noConv, seen)
				}
			}
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil, nil
	}
	if opts.conv != nil {
		t := rv.Type()
		if entry, ok := opts.conv.lookupConcrete(t); ok {
			noConv := opts
			noConv.conv = nil
			return convertAny(entry.convertReflect(rv), listCtx, depth, noConv, seen)
		}
		if entry, ok := opts.conv.lookupIface(t); ok {
			noConv := opts
			noConv.conv = nil
			return convertAny(entry.convertReflect(rv), listCtx, depth, noConv, seen)
		}
	}
	if v, handled := normalizeLeafReflect(rv); handled {
		return v, nil
	}
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			// Typed nil slices passed to array operators ($in/$nin/$and/$or) behave
			// identically to empty slices instead of failing with got <nil>.
			if listCtx && !isKeyValueStruct(rv.Type().Elem()) {
				return make([]any, 0), nil
			}
			return nil, nil
		}
		isKV := isKeyValueStruct(rv.Type().Elem())
		if rv.Len() == 0 {
			if isKV {
				return map[string]any{}, nil
			}
			return make([]any, 0), nil
		}
		k := cycleKey{kind: reflect.Slice, ptr: rv.Pointer(), n: rv.Len()}
		if seen[k] {
			return nil, invalidCondition("condition contains a cyclic reference")
		}
		seen[k] = true
		pushed = append(pushed, k)
		if isKV {
			return convertConditionWithSeen(orderedDocToMap(rv), depth, opts, seen)
		}
		return convertReflectSlice(rv, depth, opts, seen)
	case reflect.Array:
		return convertReflectSlice(rv, depth, opts, seen)
	case reflect.Map:
		if rv.IsNil() {
			return nil, nil
		}
		if rv.Len() == 0 {
			return map[string]any{}, nil
		}
		k := cycleKey{kind: reflect.Map, ptr: rv.Pointer()}
		if seen[k] {
			return nil, invalidCondition("condition contains a cyclic reference")
		}
		seen[k] = true
		pushed = append(pushed, k)
		return convertConditionWithSeen(reflectMapToAny(rv), depth, opts, seen)
	case reflect.Struct:
		return convertConditionWithSeen(structToMap(rv), depth, opts, seen)
	}
	return rv.Interface(), nil
}

func convertReflectSlice(rv reflect.Value, depth int, opts normOpts, seen map[cycleKey]bool) ([]any, error) {
	out := make([]any, rv.Len())
	for i := range out {
		v, err := convertListElem(rv.Index(i).Interface(), depth, opts, seen)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// expandParts turns ["a", "b.c"] into {"a": {"b.c": value}}.
func expandParts(parts []string, value any) map[string]any {
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
