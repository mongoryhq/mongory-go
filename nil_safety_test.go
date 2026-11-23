package mongory_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/mongoryhq/mongory-go"
)

func TestNilRegexpCondition(t *testing.T) {
	var nilRe *regexp.Regexp

	// Literal nil *regexp.Regexp normalizes to nil (null/missing semantics) and never panics.
	m, err := mongory.Compile(M{"name": nilRe})
	if err != nil {
		t.Fatalf("Compile literal nil *regexp.Regexp: %v", err)
	}
	if !m.Match(M{"name": nil}) {
		t.Fatalf("expected match on explicit nil")
	}
	if !m.Match(M{}) {
		t.Fatalf("expected match on missing field")
	}
	if m.Match(M{"name": "abc"}) {
		t.Fatalf("unexpected match on non-nil string")
	}
	type named struct {
		Name string `bson:"name"`
	}
	if m.Match(named{Name: "abc"}) {
		t.Fatalf("unexpected match on struct string field")
	}
	_ = m.Explain()
	if matched, _ := m.Trace(M{"name": "abc"}); matched {
		t.Fatalf("unexpected trace match")
	}

	// Multi-key map triggers sortByPriority during Compile.
	mMulti, err := mongory.Compile(M{"a": 1, "b": nilRe})
	if err != nil {
		t.Fatalf("Compile multi-key with nil *regexp.Regexp: %v", err)
	}
	if !mMulti.Match(M{"a": 1, "b": nil}) {
		t.Fatalf("expected multi-key match")
	}
	if mMulti.Match(M{"a": 1, "b": "x"}) {
		t.Fatalf("unexpected multi-key match")
	}

	// Array record path with nil *regexp.Regexp.
	mArr, err := mongory.Compile(M{"tags": nilRe})
	if err != nil {
		t.Fatalf("Compile array literal nil *regexp.Regexp: %v", err)
	}
	if !mArr.Match(M{"tags": A{"a", nil}}) {
		t.Fatalf("expected array with nil element to match")
	}
	if mArr.Match(M{"tags": A{"a", "b"}}) {
		t.Fatalf("unexpected match on array without nil")
	}

	// Explicit $regex operator rejects nil *regexp.Regexp.
	if _, err := mongory.Compile(M{"name": M{"$regex": nilRe}}); !errors.Is(err, mongory.ErrInvalidCondition) {
		t.Fatalf("Compile $regex nil *regexp.Regexp: got %v, want ErrInvalidCondition", err)
	}
}

type nilSafetyAmountValuer interface {
	AmountCents() int64
}

type nilSafetyMoney struct {
	cents int64
}

func (m nilSafetyMoney) AmountCents() int64 { return m.cents }

type nilSafetyOrder struct {
	ID     int            `bson:"id"`
	Amount nilSafetyMoney `bson:"amount"`
}

func TestWithConverterInterfaceAddressableField(t *testing.T) {
	opt := mongory.WithConverter[nilSafetyAmountValuer](func(v nilSafetyAmountValuer) any {
		return v.AmountCents()
	})
	m, err := mongory.Compile(M{"amount": M{"$gte": 500}}, opt)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	orders := []nilSafetyOrder{
		{ID: 1, Amount: nilSafetyMoney{cents: 100}},
		{ID: 2, Amount: nilSafetyMoney{cents: 500}},
		{ID: 3, Amount: nilSafetyMoney{cents: 950}},
	}
	got := mongory.Filter(orders, m)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("Filter(orders) = %+v, want IDs 2 and 3", got)
	}

	if !m.Match(&orders[2]) {
		t.Fatalf("expected pointer struct match")
	}
	if m.Match(&orders[0]) {
		t.Fatalf("unexpected pointer struct match")
	}
	if !m.Match(M{"amount": nilSafetyMoney{cents: 700}}) {
		t.Fatalf("expected map match")
	}
}

func TestNilAndZeroMatcherFailClosed(t *testing.T) {
	type item struct {
		ID int `bson:"id"`
	}
	mapRecords := []M{{"id": 1}, {"id": 2}}
	structRecords := []item{{ID: 1}, {ID: 2}}

	cases := []struct {
		name string
		m    *mongory.Matcher
	}{
		{name: "nil", m: nil},
		{name: "zero", m: &mongory.Matcher{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.m.Match(M{"id": 1}) {
				t.Fatalf("Match = true, want false")
			}
			if got := tc.m.Explain(); got != "" {
				t.Fatalf("Explain = %q, want empty", got)
			}
			if got := tc.m.String(); got != "" {
				t.Fatalf("String = %q, want empty", got)
			}
			if matched, trace := tc.m.Trace(M{"id": 1}); matched || trace != "" {
				t.Fatalf("Trace = (%v, %q), want (false, \"\")", matched, trace)
			}
			if got := mongory.Filter(mapRecords, tc.m); got != nil {
				t.Fatalf("Filter(maps) = %v, want nil", got)
			}
			if got := mongory.Filter(structRecords, tc.m); got != nil {
				t.Fatalf("Filter(structs) = %v, want nil", got)
			}
			if got := mongory.FilterLimit(structRecords, tc.m, 1); got != nil {
				t.Fatalf("FilterLimit(1) = %v, want nil", got)
			}
			if got := mongory.FilterLimit(structRecords, tc.m, -1); got != nil {
				t.Fatalf("FilterLimit(-1) = %v, want nil", got)
			}
			if v, ok := mongory.Find(structRecords, tc.m); ok || v != (item{}) {
				t.Fatalf("Find(structs) = (%+v, %v), want (zero, false)", v, ok)
			}
			if mongory.Any(mapRecords, tc.m) {
				t.Fatalf("Any = true, want false")
			}
			if got := mongory.Count(structRecords, tc.m); got != 0 {
				t.Fatalf("Count = %d, want 0", got)
			}
		})
	}
}

func TestCyclicPointerInSliceCondition(t *testing.T) {
	type cyclicPtr *cyclicPtr
	var p cyclicPtr
	p = &p

	_, err := mongory.Compile(M{"tags": A{p}})
	if !errors.Is(err, mongory.ErrInvalidCondition) {
		t.Fatalf("Compile cyclic pointer in slice: got %v, want ErrInvalidCondition", err)
	}
}
