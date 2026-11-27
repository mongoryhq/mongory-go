package engine

import (
	"maps"
	"reflect"
	"strings"
)

// And combines multiple query maps into one normalized document. Nil and empty
// (map[string]any{}) entries act as the identity element and are omitted. It
// flattens nested $and lists, merges disjoint field keys and disjoint pure-$
// operator subdocuments without mutating any caller map, deduplicates identical
// clauses via reflect.DeepEqual, and preserves key conflicts inside $and.
func And(conds ...map[string]any) map[string]any {
	flat := make([]map[string]any, 0, len(conds))
	for _, c := range conds {
		appendFlattenedAnd(&flat, c)
	}
	merged := make(map[string]any)
	var conflicts []map[string]any

	for _, clause := range flat {
		for _, k := range sortedKeys(clause) {
			sv := clause[k]
			dv, exists := merged[k]
			if !exists {
				merged[k] = sv
				continue
			}
			if reflect.DeepEqual(dv, sv) {
				continue
			}
			if dm, okD := asPureOpMap(dv); okD {
				if sm, okS := asPureOpMap(sv); okS && mapsCompatible(dm, sm) {
					merged[k] = mergeNewMaps(dm, sm)
					continue
				}
			}
			appendUniqueDoc(&conflicts, map[string]any{k: sv})
		}
	}

	if len(conflicts) == 0 {
		return merged
	}
	if len(merged) == 0 && len(conflicts) == 1 {
		return maps.Clone(conflicts[0])
	}
	andList := make([]any, len(conflicts))
	for i, c := range conflicts {
		andList[i] = c
	}
	merged["$and"] = andList
	return merged
}

// Or combines multiple query maps into a normalized disjunction. Nil (including
// typed nil map[string]any(nil)) arguments are skipped as omitted branches, while
// any explicit non-nil empty map M{} represents MongoDB match-all and absorbs
// the entire Or to M{}. Nested $or lists are flattened and deduplicated. When
// no non-nil branches remain, Or returns M{"$or": []any{}} (fail-closed).
func Or(conds ...map[string]any) map[string]any {
	var branches []map[string]any
	for _, c := range conds {
		if absorbed := appendFlattenedOr(&branches, c); absorbed {
			return map[string]any{}
		}
	}
	if len(branches) == 0 {
		return map[string]any{"$or": []any{}}
	}
	if len(branches) == 1 {
		return branches[0]
	}
	out := make([]any, len(branches))
	for i, b := range branches {
		out[i] = b
	}
	return map[string]any{"$or": out}
}

func appendFlattenedAnd(out *[]map[string]any, c map[string]any) {
	if len(c) == 0 {
		return
	}
	rawAnd, hasAnd := c["$and"]
	if !hasAnd {
		*out = append(*out, c)
		return
	}
	subDocs, ok := asDocSlice(rawAnd)
	if !ok {
		*out = append(*out, c)
		return
	}
	if len(c) > 1 {
		rest := make(map[string]any, len(c)-1)
		for k, v := range c {
			if k != "$and" {
				rest[k] = v
			}
		}
		*out = append(*out, rest)
	}
	for _, sub := range subDocs {
		appendFlattenedAnd(out, sub)
	}
}

func appendFlattenedOr(out *[]map[string]any, c map[string]any) bool {
	if c == nil {
		return false
	}
	if len(c) == 0 {
		return true
	}
	if len(c) == 1 {
		if rawOr, hasOr := c["$or"]; hasOr {
			if subDocs, ok := asDocSlice(rawOr); ok {
				for _, sub := range subDocs {
					if appendFlattenedOr(out, sub) {
						return true
					}
				}
				return false
			}
		}
	}
	appendUniqueDoc(out, maps.Clone(c))
	return false
}

func appendUniqueDoc(out *[]map[string]any, doc map[string]any) {
	for _, existing := range *out {
		if reflect.DeepEqual(existing, doc) {
			return
		}
	}
	*out = append(*out, doc)
}

func asStringDoc(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	if rv.IsNil() {
		return nil, true
	}
	return reflectMapToAny(rv), true
}

func asDocSlice(v any) ([]map[string]any, bool) {
	switch s := v.(type) {
	case []any:
		out := make([]map[string]any, len(s))
		for i, elem := range s {
			doc, ok := asStringDoc(elem)
			if !ok {
				return nil, false
			}
			out[i] = doc
		}
		return out, true
	case []map[string]any:
		out := make([]map[string]any, len(s))
		copy(out, s)
		return out, true
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Kind() != reflect.Slice || isKeyValueStruct(rv.Type().Elem()) {
		return nil, false
	}
	out := make([]map[string]any, rv.Len())
	for i := range rv.Len() {
		doc, ok := asStringDoc(rv.Index(i).Interface())
		if !ok {
			return nil, false
		}
		out[i] = doc
	}
	return out, true
}

// asPureOpMap matches non-empty string-keyed maps whose keys are all "$"
// operators other than "$regex"/"$options" (which are semantically coupled and
// must never be merged across clauses by key disjointness alone).
func asPureOpMap(v any) (map[string]any, bool) {
	m, ok := asStringDoc(v)
	if !ok || len(m) == 0 {
		return nil, false
	}
	for k := range m {
		if !strings.HasPrefix(k, "$") || k == "$regex" || k == "$options" {
			return nil, false
		}
	}
	return m, true
}

func mapsCompatible(a, b map[string]any) bool {
	for k, bv := range b {
		if av, exists := a[k]; exists && !reflect.DeepEqual(av, bv) {
			return false
		}
	}
	return true
}

func mergeNewMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	maps.Copy(out, a)
	maps.Copy(out, b)
	return out
}

// mergeExpandedNode merges src (a single dotted-path expansion tree) into dst
// during condition normalization. Non-colliding sub-objects merge recursively;
// leaf collisions return the non-merged residual branch so the caller appends
// it as an explicit $and clause instead of silently overwriting.
func mergeExpandedNode(dst, src map[string]any) map[string]any {
	var residual map[string]any
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		if reflect.DeepEqual(dv, sv) {
			continue
		}
		dm, dok := dv.(map[string]any)
		sm, sok := sv.(map[string]any)
		if dok && sok {
			if subResidual := mergeExpandedNode(dm, sm); len(subResidual) > 0 {
				if residual == nil {
					residual = make(map[string]any)
				}
				residual[k] = subResidual
			}
			continue
		}
		if residual == nil {
			residual = make(map[string]any)
		}
		residual[k] = sv
	}
	return residual
}
