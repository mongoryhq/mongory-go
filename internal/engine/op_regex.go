package engine

import (
	"regexp"
	"strings"
)

func init() {
	register("$regex", func(c any) (matcher, error) { return buildRegex(c, nil) })
}

// regexMatcher matches string values only.
type regexMatcher struct {
	re *regexp.Regexp
}

// buildRegex accepts a pattern string or *regexp.Regexp, plus the optional
// MongoDB "$options" sibling (i, m, s; RE2 has no x).
func buildRegex(condition, options any) (matcher, error) {
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
	switch c := condition.(type) {
	case *regexp.Regexp:
		if flags != "" {
			return nil, invalidCondition("$options cannot modify a compiled *regexp.Regexp")
		}
		return regexMatcher{re: c}, nil
	case string:
		pattern := c
		if flags != "" {
			pattern = "(?" + flags + ")" + c
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, invalidCondition("$regex: %v", err)
		}
		return regexMatcher{re: re}, nil
	}
	return nil, invalidCondition("$regex needs a string or *regexp.Regexp, got %T", condition)
}

func (m regexMatcher) match(record any) bool {
	s, ok := record.(string)
	return ok && m.re.MatchString(s)
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
