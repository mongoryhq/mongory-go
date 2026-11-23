package engine

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unsafe"
)

// ErrInvalidOption is returned by Compile (and panicked on by MustCompile) when
// an Option has an invalid argument such as a nil callback, invalid operator
// name, built-in operator override, or unsupported type parameter. It does not
// wrap ErrInvalidCondition so callers can distinguish configuration mistakes from
// untrusted query input errors.
var ErrInvalidOption = errors.New("mongory: invalid option")

func invalidOption(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidOption}, args...)...)
}

// Option configures Compile behavior.
type Option interface {
	apply(*CompileConfig)
}

type optionFunc func(*CompileConfig)

func (f optionFunc) apply(c *CompileConfig) { f(c) }

// CompileConfig controls compile-time limits and per-Compile extensions.
type CompileConfig struct {
	MaxDepth int
	MaxNodes int

	optErrs    []error
	converters *converterRegistry
	getters    *getterRegistry
	operators  map[string]Operator
}

func (c *CompileConfig) addOptionError(err error) {
	c.optErrs = append(c.optErrs, err)
}

func (c *CompileConfig) validateError() error {
	if len(c.optErrs) == 0 {
		return nil
	}
	return errors.Join(c.optErrs...)
}

// WithOptions bundles multiple options into one reusable Option, applying them
// from left to right (later options override earlier duplicates). Nil entries
// are ignored.
func WithOptions(opts ...Option) Option {
	cloned := slices.Clone(opts)
	return optionFunc(func(c *CompileConfig) {
		for _, opt := range cloned {
			if opt != nil {
				opt.apply(c)
			}
		}
	})
}

// WithMaxDepth overrides DefaultMaxDepth (64). A non-positive value disables
// the depth limit.
func WithMaxDepth(depth int) Option {
	return optionFunc(func(c *CompileConfig) { c.MaxDepth = depth })
}

// WithMaxNodes overrides DefaultMaxNodes (4096). A non-positive value disables
// the node limit.
func WithMaxNodes(nodes int) Option {
	return optionFunc(func(c *CompileConfig) { c.MaxNodes = nodes })
}

// Predicate evaluates a single custom operator against raw record data.
// When exists is false, the field is missing from its parent container and
// value is nil. When the field is present with a nil or typed-nil value,
// exists is true and value preserves the original typed value.
type Predicate func(value any, exists bool) bool

// Operator defines a compile-scoped custom query operator (name starting with "$").
type Operator struct {
	// Compile validates and optionally pre-processes the raw condition argument
	// once at Compile time, returning a concurrency-safe match Predicate.
	Compile func(arg any) (Predicate, error)
	// MatchElements instructs implicit array matching to apply this operator
	// element-by-element (like $gt) rather than passing the whole slice (like $size).
	MatchElements bool
	// Priority hints sibling evaluation ordering inside allOf (0 defaults to 3).
	Priority int
}

// WithOperator registers a custom operator scoped strictly to this Compile call.
// Overriding built-in operators (or "$regex" / "$options") or passing a nil
// Compile function fails Compile with ErrInvalidOption.
func WithOperator(name string, op Operator) Option {
	return optionFunc(func(c *CompileConfig) {
		if !strings.HasPrefix(name, "$") || len(name) <= 1 {
			c.addOptionError(invalidOption("operator name %q must start with \"$\"", name))
			return
		}
		if isBuiltinOperator(name) {
			c.addOptionError(invalidOption("cannot override built-in operator %q", name))
			return
		}
		if op.Compile == nil {
			c.addOptionError(invalidOption("operator %q has nil Compile function", name))
			return
		}
		if c.operators == nil {
			c.operators = make(map[string]Operator)
		}
		c.operators[name] = op
	})
}

func isBuiltinOperator(name string) bool {
	if _, ok := operators[name]; ok {
		return true
	}
	return name == "$regex" || name == "$options"
}

type converterEntry struct {
	typ            reflect.Type
	convertAny     func(any) any
	convertReflect func(reflect.Value) any
}

type converterRegistry struct {
	entries []converterEntry
	byType  map[reflect.Type]int
	ifaces  []converterEntry
}

func (r *converterRegistry) add(e converterEntry) {
	if e.typ.Kind() == reflect.Interface {
		for i := range r.ifaces {
			if r.ifaces[i].typ == e.typ {
				r.ifaces[i] = e
				return
			}
		}
		r.ifaces = append(r.ifaces, e)
		return
	}
	for i := range r.entries {
		if r.entries[i].typ == e.typ {
			r.entries[i] = e
			return
		}
	}
	idx := len(r.entries)
	r.entries = append(r.entries, e)
	if len(r.entries) > 4 {
		if r.byType == nil {
			r.byType = make(map[reflect.Type]int, len(r.entries))
			for i := range r.entries {
				r.byType[r.entries[i].typ] = i
			}
		} else {
			r.byType[e.typ] = idx
		}
	}
}

func (r *converterRegistry) lookupConcrete(t reflect.Type) (converterEntry, bool) {
	if r == nil || t == nil {
		return converterEntry{}, false
	}
	if r.byType != nil {
		if idx, ok := r.byType[t]; ok {
			return r.entries[idx], true
		}
		return converterEntry{}, false
	}
	for i := range r.entries {
		if r.entries[i].typ == t {
			return r.entries[i], true
		}
	}
	return converterEntry{}, false
}

func (r *converterRegistry) lookupIface(t reflect.Type) (converterEntry, bool) {
	if r == nil || t == nil || len(r.ifaces) == 0 {
		return converterEntry{}, false
	}
	for i := range r.ifaces {
		if t.Implements(r.ifaces[i].typ) {
			return r.ifaces[i], true
		}
	}
	return converterEntry{}, false
}

// applyAny applies a registered converter to v and returns the raw (unnormalized)
// converter output alongside matched=true. Nil pointers/interfaces never invoke
// user callbacks and return (nil, false).
func (r *converterRegistry) applyAny(v any) (any, bool) {
	if r == nil || v == nil {
		return nil, false
	}
	if _, ok := v.(missingKey); ok {
		return nil, false
	}
	t := reflect.TypeOf(v)
	if entry, ok := r.lookupConcrete(t); ok {
		if t.Kind() == reflect.Pointer && reflect.ValueOf(v).IsNil() {
			return nil, false
		}
		return entry.convertAny(v), true
	}
	if entry, ok := r.lookupIface(t); ok {
		if t.Kind() == reflect.Pointer && reflect.ValueOf(v).IsNil() {
			return nil, false
		}
		return entry.convertAny(v), true
	}
	if t.Kind() == reflect.Pointer {
		return r.applyReflect(reflect.ValueOf(v))
	}
	return nil, false
}

// applyReflect walks pointers/interfaces (bounded by maxPointerDepth) and runs
// the first matching converter on the pointer type first (*X) then unwrapped X,
// then interface registrations. Nil pointers return (nil, false) before running fn.
func (r *converterRegistry) applyReflect(rv reflect.Value) (any, bool) {
	if r == nil || !rv.IsValid() {
		return nil, false
	}
	depth := 0
	for rv.IsValid() {
		switch rv.Kind() {
		case reflect.Pointer, reflect.Interface:
			if rv.IsNil() {
				return nil, false
			}
			if rv.Kind() == reflect.Pointer {
				t := rv.Type()
				if entry, ok := r.lookupConcrete(t); ok {
					return entry.convertReflect(rv), true
				}
				if entry, ok := r.lookupIface(t); ok {
					return entry.convertReflect(rv), true
				}
			}
			depth++
			if depth > maxPointerDepth {
				return nil, false
			}
			rv = rv.Elem()
		default:
			t := rv.Type()
			if entry, ok := r.lookupConcrete(t); ok {
				return entry.convertReflect(rv), true
			}
			if entry, ok := r.lookupIface(t); ok {
				return entry.convertReflect(rv), true
			}
			return nil, false
		}
	}
	return nil, false
}

// applyNormalized converts and canonicalizes a runtime record value using the
// converter registry without re-entering converters on the output.
func (r *converterRegistry) applyNormalized(v any) (any, bool) {
	out, ok := r.applyAny(v)
	if !ok {
		return nil, false
	}
	return normalize(out), true
}

func (r *converterRegistry) applyReflectNormalized(rv reflect.Value) (any, bool) {
	out, ok := r.applyReflect(rv)
	if !ok {
		return nil, false
	}
	return normalize(out), true
}

// WithTypeConverter registers a symmetric converter for values of type T across
// built-in operators and comparisons. Custom operators still receive raw values.
// T must not be the empty interface `any`.
func WithTypeConverter[T any](fn func(T) any) Option {
	typ := reflect.TypeFor[T]()
	return optionFunc(func(c *CompileConfig) {
		if fn == nil {
			c.addOptionError(invalidOption("WithConverter: nil function for type %v", typ))
			return
		}
		if typ == nil || (typ.Kind() == reflect.Interface && typ.NumMethod() == 0) {
			c.addOptionError(invalidOption("WithConverter: catch-all empty interface `any` is not supported"))
			return
		}
		if c.converters == nil {
			c.converters = &converterRegistry{}
		}
		c.converters.add(converterEntry{
			typ: typ,
			convertAny: func(v any) any {
				return fn(v.(T))
			},
			convertReflect: func(rv reflect.Value) any {
				if rv.CanAddr() && rv.Type() == typ && rv.CanInterface() {
					return fn(*(*T)(unsafe.Pointer(rv.Addr().UnsafePointer())))
				}
				if rv.CanInterface() {
					return fn(rv.Interface().(T))
				}
				return nil
			},
		})
	})
}

type boundFieldGetter struct {
	needPtr bool
	invoke  func(elemRV reflect.Value, needPtr bool) any
}

type boundFieldResolver struct {
	needPtr bool
	invoke  func(elemRV reflect.Value, needPtr bool, field string) (any, bool)
}

type fieldGetterEntry struct {
	seq     int
	typ     reflect.Type
	isIface bool
	isPtr   bool
	elemTyp reflect.Type
	field   string
	invoke  func(elemRV reflect.Value, needPtr bool) any
}

type fieldResolverEntry struct {
	seq     int
	typ     reflect.Type
	isIface bool
	isPtr   bool
	elemTyp reflect.Type
	invoke  func(elemRV reflect.Value, needPtr bool, field string) (any, bool)
}

type getterRegistry struct {
	nextSeq   int
	fields    []fieldGetterEntry
	resolvers []fieldResolverEntry
}

func validateCustomTargetType(optName string, typ reflect.Type) error {
	if typ == nil || (typ.Kind() == reflect.Interface && typ.NumMethod() == 0) {
		return invalidOption("%s: catch-all empty interface `any` is not supported", optName)
	}
	elem := typ
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() == reflect.Interface && elem.NumMethod() == 0 {
		return invalidOption("%s: pointer to empty interface is not supported", optName)
	}
	if elem.Kind() != reflect.Struct && elem.Kind() != reflect.Interface && elem.PkgPath() == "" {
		return invalidOption("%s: canonical unnamed type %v is not supported", optName, typ)
	}
	return nil
}

func extractTypedArg[T any](elemRV reflect.Value, needPtr bool) (T, bool) {
	typ := reflect.TypeFor[T]()
	if needPtr {
		if elemRV.CanAddr() && elemRV.CanInterface() {
			if v, ok := elemRV.Addr().Interface().(T); ok {
				return v, true
			}
		}
		if elemRV.CanInterface() {
			cp := reflect.New(elemRV.Type())
			cp.Elem().Set(elemRV)
			if v, ok := cp.Interface().(T); ok {
				return v, true
			}
		}
		var zero T
		return zero, false
	}
	if elemRV.CanAddr() && elemRV.Type() == typ && elemRV.CanInterface() {
		return *(*T)(unsafe.Pointer(elemRV.Addr().UnsafePointer())), true
	}
	if elemRV.CanInterface() {
		if v, ok := elemRV.Interface().(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func makeTypedValueInvoker[T any](fn func(T) any) func(elemRV reflect.Value, needPtr bool) any {
	return func(elemRV reflect.Value, needPtr bool) any {
		arg, ok := extractTypedArg[T](elemRV, needPtr)
		if !ok {
			return missing
		}
		return fn(arg)
	}
}

func makeTypedResolverInvoker[T any](fn func(T, string) (any, bool)) func(elemRV reflect.Value, needPtr bool, field string) (any, bool) {
	return func(elemRV reflect.Value, needPtr bool, field string) (any, bool) {
		arg, ok := extractTypedArg[T](elemRV, needPtr)
		if !ok {
			return nil, false
		}
		return fn(arg, field)
	}
}

// WithField binds an explicit getter function to a single field segment on
// values matching T (concrete type, pointer *T, or interface T).
func WithField[T any](name string, get func(T) any) Option {
	typ := reflect.TypeFor[T]()
	return optionFunc(func(c *CompileConfig) {
		if name == "" || strings.HasPrefix(name, "$") {
			c.addOptionError(invalidOption("WithField: invalid field name %q", name))
			return
		}
		if get == nil {
			c.addOptionError(invalidOption("WithField: nil getter for field %q", name))
			return
		}
		if err := validateCustomTargetType("WithField", typ); err != nil {
			c.addOptionError(err)
			return
		}
		if c.getters == nil {
			c.getters = &getterRegistry{}
		}
		c.getters.nextSeq++
		var elemTyp reflect.Type
		if typ.Kind() == reflect.Pointer {
			elemTyp = typ.Elem()
		}
		c.getters.fields = append(c.getters.fields, fieldGetterEntry{
			seq:     c.getters.nextSeq,
			typ:     typ,
			isIface: typ.Kind() == reflect.Interface,
			isPtr:   typ.Kind() == reflect.Pointer,
			elemTyp: elemTyp,
			field:   name,
			invoke:  makeTypedValueInvoker(get),
		})
	})
}

// WithFieldResolver registers a dynamic per-segment resolver for values matching
// T (concrete type, pointer *T, or interface T). Returning ok=false falls back
// to earlier resolvers and then struct tag / map lookup.
func WithFieldResolver[T any](fn func(rec T, field string) (val any, ok bool)) Option {
	typ := reflect.TypeFor[T]()
	return optionFunc(func(c *CompileConfig) {
		if fn == nil {
			c.addOptionError(invalidOption("WithFieldResolver: nil function for type %v", typ))
			return
		}
		if err := validateCustomTargetType("WithFieldResolver", typ); err != nil {
			c.addOptionError(err)
			return
		}
		if c.getters == nil {
			c.getters = &getterRegistry{}
		}
		c.getters.nextSeq++
		var elemTyp reflect.Type
		if typ.Kind() == reflect.Pointer {
			elemTyp = typ.Elem()
		}
		c.getters.resolvers = append(c.getters.resolvers, fieldResolverEntry{
			seq:     c.getters.nextSeq,
			typ:     typ,
			isIface: typ.Kind() == reflect.Interface,
			isPtr:   typ.Kind() == reflect.Pointer,
			elemTyp: elemTyp,
			invoke:  makeTypedResolverInvoker(fn),
		})
	})
}

func matchTargetType(entryTyp reflect.Type, isIface, isPtr bool, elemTyp, concreteType, ptrConcreteType reflect.Type) (matched bool, needPtr bool) {
	if isIface {
		if concreteType.Implements(entryTyp) {
			return true, false
		}
		if ptrConcreteType.Implements(entryTyp) {
			return true, true
		}
		return false, false
	}
	if isPtr {
		return elemTyp == concreteType, true
	}
	return entryTyp == concreteType, false
}

// resolveFor returns the matching static getter (if any) and ordered dynamic
// resolvers (most recently registered first) for (concreteType, field).
func (r *getterRegistry) resolveFor(concreteType reflect.Type, field string) (*boundFieldGetter, []boundFieldResolver) {
	if r == nil || concreteType == nil {
		return nil, nil
	}
	ptrType := reflect.PointerTo(concreteType)
	var staticBest *boundFieldGetter
	// Static fields beat dynamic resolvers; within static fields, exact concrete/*T
	// beats interface match, and latest registration wins ties.
	bestRank := -1
	bestSeq := -1
	for i := range r.fields {
		e := &r.fields[i]
		if e.field != field {
			continue
		}
		matched, needPtr := matchTargetType(e.typ, e.isIface, e.isPtr, e.elemTyp, concreteType, ptrType)
		if !matched {
			continue
		}
		rank := 1
		if e.isIface {
			rank = 0
		}
		if rank > bestRank || (rank == bestRank && e.seq > bestSeq) {
			bestRank = rank
			bestSeq = e.seq
			staticBest = &boundFieldGetter{needPtr: needPtr, invoke: e.invoke}
		}
	}
	var resolvers []boundFieldResolver
	for i := len(r.resolvers) - 1; i >= 0; i-- {
		e := r.resolvers[i]
		if matched, needPtr := matchTargetType(e.typ, e.isIface, e.isPtr, e.elemTyp, concreteType, ptrType); matched {
			resolvers = append(resolvers, boundFieldResolver{needPtr: needPtr, invoke: e.invoke})
		}
	}
	return staticBest, resolvers
}
