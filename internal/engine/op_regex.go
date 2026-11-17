package engine

import (
	"reflect"
	"regexp"
	"strings"
)

func init() {
	register("$regex", func(ctx *compileCtx, c any) (matcher, error) { return buildRegex(ctx, c, nil) })
}

// regexMatcher matches string values only.
type regexMatcher struct {
	re *regexp.Regexp
}

// buildRegex accepts a pattern string or *regexp.Regexp, plus the optional
// MongoDB "$options" sibling (i, m, s; RE2 has no x).
func buildRegex(ctx *compileCtx, condition, options any) (matcher, error) {
	var flags string
	if options != nil {
		s, ok := options.(string)
		if !ok {
			return nil, invalidCondition("$options needs a string, got %T", options)
		}
		for _, f := range s {
			if !strings.ContainsRune("ims", f) {
				return nil, invalidCondition("unsupported $options flag %q", f)
			}
		}
		flags = s
	}
	wrap := func(re *regexp.Regexp) (matcher, error) {
		m, err := addNodeMatcher(ctx, regexMatcher{re: re})
		if err != nil {
			return nil, err
		}
		if conv := ctx.convOrNil(); conv != nil {
			return convValueMatcher{conv: conv, inner: m}, nil
		}
		return m, nil
	}
	switch c := condition.(type) {
	case *regexp.Regexp:
		if flags != "" {
			return nil, invalidCondition("$options cannot modify a compiled *regexp.Regexp")
		}
		return wrap(c)
	case string:
		pattern := c
		if flags != "" {
			pattern = "(?" + flags + ")" + c
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, invalidCondition("$regex: %v", err)
		}
		return wrap(re)
	}
	return nil, invalidCondition("$regex needs a string or *regexp.Regexp, got %T", condition)
}

func (m regexMatcher) match(record any) bool {
	s, ok := record.(string)
	return ok && m.re.MatchString(s)
}

func (m regexMatcher) matchReflect(rv reflect.Value) bool {
	rv, ok := unwrapReflect(rv)
	if !ok || rv.Kind() != reflect.String {
		return false
	}
	if rv.Type() == jsonNumberType {
		return m.match(parseJSONNumberString(rv.String()))
	}
	if hasTimeValuer(rv.Type()) {
		return false
	}
	return m.re.MatchString(rv.String())
}

func (m regexMatcher) priority() int {
	if strings.HasPrefix(m.re.String(), "^") {
		return 8
	}
	return 20
}

func (m regexMatcher) label() string { return "Regex(/" + m.re.String() + "/)" }
func (m regexMatcher) explain(buf *strings.Builder, prefix string, isLast, isRoot bool) {
	writeTreeLine(buf, prefix, isLast, isRoot, m.label())
}
func (m regexMatcher) trace(record any, buf *strings.Builder, prefix string, isLast, isRoot bool) bool {
	return traceLeaf(m, record, buf, prefix, isLast, isRoot, m.label())
}
