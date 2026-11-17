package mongory_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mongoryhq/mongory-go"
)

type testMoney struct {
	mu           sync.Mutex // guards against accidental value copies
	CurrencyCode string     `json:"currency_code"`
	Units        int64      `json:"units"`
	Nanos        int32      `json:"nanos"`
}

func (m *testMoney) minorUnits() int64 {
	return m.Units*1_000_000_000 + int64(m.Nanos)
}

func (m *testMoney) String() string {
	return fmt.Sprintf("%d.%09d %s", m.Units, m.Nanos, m.CurrencyCode)
}

type testOID [4]byte

func (o testOID) Hex() string {
	return string(o[:])
}

type hexer interface {
	Hex() string
}

type nullableString struct {
	String string
	Valid  bool
}

type namedAttrBag map[string]string

type dynRecord interface {
	Get(key string) (any, bool)
}

type ptrGetterRecord struct {
	attrs map[string]any
}

func (p *ptrGetterRecord) Get(key string) (any, bool) {
	v, ok := p.attrs[key]
	return v, ok
}

type promoCondModel struct {
	PromotionID testOID        `bson:"promotion_id"`
	AddonID     *testOID       `bson:"addon_id"`
	MinPrice    *testMoney     `bson:"min_price"`
	Tags        []testOID      `bson:"tags"`
	Nick        nullableString `bson:"nick"`
	Hidden      string         `bson:"-"`
	Extra       map[string]any `bson:",inline"`
}

func newPromoFixture(id testOID, addon *testOID, money *testMoney, tags ...testOID) promoCondModel {
	return promoCondModel{
		PromotionID: id,
		AddonID:     addon,
		MinPrice:    money,
		Tags:        tags,
		Hidden:      "secret-ok",
	}
}

func TestOptionsValidationAndComposition(t *testing.T) {
	badCases := []struct {
		name string
		opt  mongory.Option
	}{
		{"op missing dollar", mongory.WithOperator("eq", mongory.Operator{Compile: func(any) (mongory.Predicate, error) { return func(any, bool) bool { return true }, nil }})},
		{"op bare dollar", mongory.WithOperator("$", mongory.Operator{Compile: func(any) (mongory.Predicate, error) { return func(any, bool) bool { return true }, nil }})},
		{"op override $eq", mongory.WithOperator("$eq", mongory.Operator{Compile: func(any) (mongory.Predicate, error) { return func(any, bool) bool { return true }, nil }})},
		{"op override $regex", mongory.WithOperator("$regex", mongory.Operator{Compile: func(any) (mongory.Predicate, error) { return func(any, bool) bool { return true }, nil }})},
		{"op override $options", mongory.WithOperator("$options", mongory.Operator{Compile: func(any) (mongory.Predicate, error) { return func(any, bool) bool { return true }, nil }})},
		{"op nil compile", mongory.WithOperator("$custom", mongory.Operator{})},
		{"converter nil func", mongory.WithConverter[testOID](nil)},
		{"converter empty interface", mongory.WithConverter[any](func(v any) any { return v })},
		{"field empty name", mongory.WithField[*promoCondModel]("", func(p *promoCondModel) any { return nil })},
		{"field dollar name", mongory.WithField[*promoCondModel]("$x", func(p *promoCondModel) any { return nil })},
		{"field nil getter", mongory.WithField[*promoCondModel]("x", nil)},
		{"field any target", mongory.WithField[any]("x", func(a any) any { return a })},
		{"field canonical map", mongory.WithField[M]("x", func(m M) any { return m["x"] })},
		{"field canonical slice", mongory.WithField[A]("x", func(a A) any { return len(a) })},
		{"field canonical int", mongory.WithField[int]("x", func(i int) any { return i })},
		{"field canonical *int", mongory.WithField[*int]("x", func(i *int) any { return i })},
		{"field pointer to any", mongory.WithField[*any]("x", func(a *any) any { return *a })},
		{"resolver nil func", mongory.WithFieldResolver[namedAttrBag](nil)},
		{"resolver any target", mongory.WithFieldResolver[any](func(any, string) (any, bool) { return nil, false })},
		{"resolver canonical map", mongory.WithFieldResolver[map[string]any](func(m map[string]any, k string) (any, bool) { return m[k], true })},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mongory.Compile(M{"a": 1}, tc.opt)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, mongory.ErrInvalidOption) {
				t.Fatalf("expected ErrInvalidOption, got %v", err)
			}
			if errors.Is(err, mongory.ErrInvalidCondition) {
				t.Fatalf("ErrInvalidOption must not wrap ErrInvalidCondition: %v", err)
			}
			didPanic := false
			func() {
				defer func() {
					if recover() != nil {
						didPanic = true
					}
				}()
				_ = mongory.MustCompile(nil, tc.opt)
			}()
			if !didPanic {
				t.Fatal("MustCompile(nil, badOpt) did not panic")
			}
		})
	}

	t.Run("preset composition, defensive clone, and last-wins overrides", func(t *testing.T) {
		sliceOpts := []mongory.Option{
			mongory.WithConverter(func(o testOID) any { return "v1-" + o.Hex() }),
			mongory.WithOperator("$tagEq", mongory.Operator{
				Compile: func(arg any) (mongory.Predicate, error) {
					want := arg.(string)
					return func(v any, exists bool) bool { return exists && v == "v1-"+want }, nil
				},
			}),
		}
		preset := mongory.WithOptions(nil, sliceOpts[0], sliceOpts[1], nil)
		// Mutate caller slice after constructing preset: preset must stay unchanged.
		sliceOpts[0] = mongory.WithConverter(func(o testOID) any { return "mutated" })

		// Startup validation idiom with nil condition:
		_ = mongory.MustCompile(nil, preset)

		// Last-wins override appended after preset:
		override := mongory.WithOptions(
			preset,
			mongory.WithConverter(func(o testOID) any { return o.Hex() }),
			mongory.WithOperator("$tagEq", mongory.Operator{
				Compile: func(arg any) (mongory.Predicate, error) {
					want := arg.(string)
					return func(v any, exists bool) bool { return exists && v == want }, nil
				},
			}),
			mongory.WithMaxDepth(8),
		)

		oid := testOID{'a', 'b', 'c', 'd'}
		rec := promoCondModel{PromotionID: oid, Extra: M{"code": "ok"}}
		m := mongory.MustCompile(
			M{"promotion_id": "abcd", "code": M{"$tagEq": "ok"}},
			override,
			mongory.WithMaxNodes(16),
		)
		if !m.Match(&rec) {
			t.Fatalf("expected preset+override match; explain:\n%s", m.Explain())
		}
		// Base preset still has "v1-" semantics and wasn't polluted by override.
		mBase := mongory.MustCompile(M{"promotion_id": "v1-abcd", "code": M{"$tagEq": "ok"}}, preset)
		if mBase.Match(&rec) {
			t.Fatal("base preset should not match override's non-prefixed code")
		}
		baseRec := promoCondModel{PromotionID: oid, Extra: M{"code": "v1-ok"}}
		if !mBase.Match(&baseRec) {
			t.Fatal("expected base preset to remain isolated and match v1-prefixed record")
		}
	})
}

func TestWithConverter(t *testing.T) {
	oid1 := testOID{'p', 'r', 'o', '1'}
	oid2 := testOID{'p', 'r', 'o', '2'}
	oid3 := testOID{'p', 'r', 'o', '3'}

	type wrappedRange struct{ Min, Max int64 }

	preset := mongory.WithOptions(
		mongory.WithConverter(func(o testOID) any { return o.Hex() }),
		mongory.WithConverter(func(m *testMoney) any { return m.minorUnits() }),
		mongory.WithConverter(func(ns nullableString) any {
			if !ns.Valid {
				return nil
			}
			return ns.String
		}),
		mongory.WithConverter(func(r wrappedRange) any {
			return M{"$gte": r.Min, "$lte": r.Max}
		}),
	)

	t.Run("symmetric condition and record conversion including protobuf ptr and O(1) $in", func(t *testing.T) {
		targetMoney := &testMoney{CurrencyCode: "TWD", Units: 100, Nanos: 500_000_000}

		// B1 regression: condition uses struct ptr & slice of testOID, plus >16 items for inLookup.
		inTargets := make([]testOID, 20)
		inTargets[0] = oid1
		for i := 1; i < len(inTargets); i++ {
			inTargets[i] = testOID{'x', '0', byte('a' + i/10), byte('0' + i%10)}
		}

		cond := M{
			"promotion_id": oid1, // converted on condition side too -> "pro1"
			"addon_id":     oid2, // *testOID auto-dereferenced -> "pro2"
			"min_price":    M{"$gte": targetMoney},
			"tags":         M{"$in": inTargets},
			"nick":         M{"$regex": "^VIP", "$present": true},
			// Raw dot path still navigates into *testMoney even with converter registered:
			"min_price.currency_code": "TWD",
		}
		m := mongory.MustCompile(cond, preset)

		good := newPromoFixture(oid1, &oid2, &testMoney{CurrencyCode: "TWD", Units: 101}, oid3, oid1)
		good.Nick = nullableString{String: "VIP-01", Valid: true}

		lowPrice := good
		lowPrice.MinPrice = &testMoney{CurrencyCode: "TWD", Units: 99}

		nilAddon := good
		nilAddon.AddonID = nil

		nullNick := good
		nullNick.Nick = nullableString{String: "VIP-99", Valid: false} // converts to nil

		if !m.Match(&good) || !m.Match(good) {
			t.Fatal("expected converted record match")
		}
		if m.Match(&lowPrice) || m.Match(&nilAddon) || m.Match(&nullNick) {
			t.Fatal("unexpected match on non-matching converted fields")
		}

		// Nil pointer guard + nullableString -> nil checks:
		nilCheckM := mongory.MustCompile(M{
			"addon_id": nil,
			"nick":     M{"$eq": nil, "$exists": true, "$present": false},
		}, preset)
		onlyNull := newPromoFixture(oid1, nil, nil)
		onlyNull.Nick = nullableString{Valid: false}
		if !nilCheckM.Match(&onlyNull) {
			t.Fatalf("expected typed nil addon + invalid nullableString match; trace:\n%s", func() string {
				_, s := nilCheckM.Trace(&onlyNull)
				return s
			}())
		}

		// Whole-slice $eq on []testOID + array A{testOID} record + wrappedRange in condition:
		sliceM := mongory.MustCompile(M{
			"tags":          []testOID{oid1, oid2},
			"score":         wrappedRange{Min: 10, Max: 20},
			"tags_explicit": "pro2",
		}, preset)
		recMap := M{
			"tags":          []testOID{oid1, oid2},
			"score":         15,
			"tags_explicit": A{oid1, oid2},
		}
		if !sliceM.Match(recMap) {
			t.Fatal("expected slice element conversion & wrappedRange condition match")
		}

		for _, rec := range []any{&good, good, &lowPrice, &nilAddon, &onlyNull, recMap} {
			gotM := m.Match(rec)
			gotT, tr := m.Trace(rec)
			if gotM != gotT {
				t.Fatalf("Match(%v) != Trace(%v):\n%s", gotM, gotT, tr)
			}
		}
	})

	t.Run("no converter chaining or loops", func(t *testing.T) {
		type hopA int
		type hopB string
		m := mongory.MustCompile(
			M{"v": hopB("final")},
			mongory.WithConverter(func(a hopA) any { return hopB("step1") }),
			mongory.WithConverter(func(b hopB) any { return "final" }),
		)
		// hopA converts once to hopB("step1"), does NOT chain into hopB -> "final".
		if m.Match(M{"v": hopA(1)}) {
			t.Fatal("converter must not chain onto another converter output")
		}
	})

	t.Run("interface converter & nested map/slice equality", func(t *testing.T) {
		m := mongory.MustCompile(
			M{"doc": M{"id": "pro1"}},
			mongory.WithConverter(func(h hexer) any { return h.Hex() }),
		)
		type innerDoc struct {
			ID testOID `bson:"id"`
		}
		if !m.Match(M{"doc": innerDoc{ID: oid1}}) {
			t.Fatal("expected interface converter to work on nested struct whole-doc eq")
		}
	})
}

func TestWithFieldAndResolver(t *testing.T) {
	type customItem struct {
		Base   string `bson:"base"`
		amount int64
	}
	bag := namedAttrBag{"region": "tw", "tier": "vip"}

	t.Run("static field, resolver precedence, named map, and ptr-receiver interface", func(t *testing.T) {
		var orderCalls []string
		opts := mongory.WithOptions(
			mongory.WithFieldResolver(func(b namedAttrBag, k string) (any, bool) {
				v, ok := b[k]
				return v, ok
			}),
			mongory.WithFieldResolver(func(d dynRecord, k string) (any, bool) {
				orderCalls = append(orderCalls, "r1:"+k)
				return d.Get(k)
			}),
			mongory.WithFieldResolver(func(rec *customItem, k string) (any, bool) {
				orderCalls = append(orderCalls, "r2:"+k)
				if k == "amount" {
					return rec.amount, true
				}
				return nil, false // fall back to struct tags ("base")
			}),
			mongory.WithField("amount", func(rec customItem) any {
				orderCalls = append(orderCalls, "static:amount")
				return rec.amount * 2 // WithField beats WithFieldResolver
			}),
			// Pointer-receiver interface test on non-addressable struct value:
			mongory.WithField("dyn_x", func(d dynRecord) any {
				v, _ := d.Get("x")
				return v
			}),
		)

		mItem := mongory.MustCompile(M{"amount": 200, "base": "ok"}, opts)
		it := customItem{Base: "ok", amount: 100}
		if !mItem.Match(it) || !mItem.Match(&it) {
			t.Fatal("expected WithField & tag fallback match on both value and pointer struct")
		}
		// Assert WithField ran rather than r2 override for "amount":
		for _, c := range orderCalls {
			if c == "r2:amount" {
				t.Fatalf("expected WithField to take precedence over WithFieldResolver, calls=%v", orderCalls)
			}
		}
		if ok, tr := mItem.Trace(&it); !ok {
			t.Fatalf("Trace failed on resolved struct field:\n%s", tr)
		}

		// Named map wrapper
		mBag := mongory.MustCompile(M{"region": "tw", "missing": M{"$exists": false}}, opts)
		if !mBag.Match(bag) {
			t.Fatal("expected named map resolver match")
		}
		if mBag.Match(namedAttrBag{"region": "us"}) {
			t.Fatal("unexpected named map match")
		}

		// Unaddressable value struct whose *pointer implements dynRecord interface:
		type valStructImpl struct {
			p ptrGetterRecord
		}
		mIface := mongory.MustCompile(M{"k1": "v1"}, opts)
		valRec := ptrGetterRecord{attrs: M{"k1": "v1"}}
		// Match directly on struct pointer AND copy-able struct value via map value (non-addressable):
		wrap := map[string]ptrGetterRecord{"rec": valRec}
		mNested := mongory.MustCompile(M{"rec.k1": "v1"}, opts)
		if !mIface.Match(&valRec) || !mNested.Match(wrap) {
			t.Fatal("expected pointer-receiver interface resolver to work on addressable and unaddressable structs")
		}
		_ = valStructImpl{}
	})
}

func TestWithOperatorAndCoexistence(t *testing.T) {
	twd100 := &testMoney{CurrencyCode: "TWD", Units: 100}
	twd200 := &testMoney{CurrencyCode: "TWD", Units: 200}
	usd300 := &testMoney{CurrencyCode: "USD", Units: 300}

	errMoneyArg := errors.New("expected *testMoney")

	opts := mongory.WithOptions(
		// TypeConverter collapses *testMoney to currency-blind int64:
		mongory.WithConverter(func(m *testMoney) any { return m.minorUnits() }),
		// Custom operator STILL receives raw *testMoney and preserves currency checks:
		mongory.WithOperator("$moneyGte", mongory.Operator{
			MatchElements: true,
			Priority:      2,
			Compile: func(arg any) (mongory.Predicate, error) {
				target, ok := arg.(*testMoney)
				if !ok || target == nil {
					return nil, errMoneyArg
				}
				return func(v any, exists bool) bool {
					rec, ok := v.(*testMoney)
					return exists && ok && rec != nil &&
						rec.CurrencyCode == target.CurrencyCode &&
						rec.minorUnits() >= target.minorUnits()
				}, nil
			},
		}),
		mongory.WithOperator("$isTypedNilInt", mongory.Operator{
			Compile: func(any) (mongory.Predicate, error) {
				return func(v any, exists bool) bool {
					if !exists {
						return false
					}
					p, ok := v.(*int)
					return ok && p == nil
				}, nil
			},
		}),
		mongory.WithOperator("$sumUnitsGte", mongory.Operator{
			MatchElements: false, // whole slice passed directly
			Compile: func(arg any) (mongory.Predicate, error) {
				want := int64(arg.(int))
				return func(v any, exists bool) bool {
					slice, ok := v.([]*testMoney)
					if !exists || !ok {
						return false
					}
					var sum int64
					for _, item := range slice {
						if item != nil {
							sum += item.Units
						}
					}
					return sum >= want
				}, nil
			},
		}),
	)

	t.Run("coexists with TypeConverter on same field + dotted custom op", func(t *testing.T) {
		m := mongory.MustCompile(M{
			"min_price": M{
				"$gte":      150_000_000_000, // converted units (150 TWD/USD)
				"$moneyGte": twd100,          // raw currency check (TWD >= 100)
			},
			"alt.$moneyGte": twd100, // B1 dot path on custom op
		}, opts)

		if !m.Match(M{"min_price": twd200, "alt": twd100}) {
			t.Fatal("expected TWD 200 to satisfy both $gte and $moneyGte")
		}
		if m.Match(M{"min_price": usd300, "alt": twd100}) {
			t.Fatal("expected USD 300 to fail currency-aware $moneyGte even though $gte passes")
		}
		if ok, trace := m.Trace(M{"min_price": usd300, "alt": twd100}); ok || trace == "" {
			t.Fatalf("Trace mismatch on custom op: ok=%v\n%s", ok, trace)
		}
		if ex := m.Explain(); ex == "" {
			t.Fatal("expected Explain output")
		}
	})

	t.Run("raw typed nil and exists=false distinction across struct, map, and $elemMatch", func(t *testing.T) {
		type holder struct {
			Ptr  *int
			List []*int
		}
		var nilInt *int
		five := 5

		m := mongory.MustCompile(M{"Ptr": M{"$isTypedNilInt": true}}, opts)
		if !m.Match(&holder{Ptr: nil}) || m.Match(&holder{Ptr: &five}) {
			t.Fatal("struct typed nil Ptr check failed")
		}
		if !m.Match(M{"Ptr": nilInt}) || m.Match(M{"Ptr": nil}) || m.Match(M{}) {
			t.Fatal("map must distinguish typed nil (*int)(nil), untyped nil, and missing")
		}
		// Match vs Trace must agree:
		for _, rec := range []any{&holder{Ptr: nil}, M{"Ptr": nilInt}, M{"Ptr": nil}, M{}} {
			if m.Match(rec) != func() bool { ok, _ := m.Trace(rec); return ok }() {
				t.Fatalf("Trace != Match on %#v", rec)
			}
		}

		// $elemMatch over []*int and []any containing typed-nil *int:
		elemM := mongory.MustCompile(M{"List": M{"$elemMatch": M{"$isTypedNilInt": true}}}, opts)
		if !elemM.Match(&holder{List: []*int{&five, nil}}) || elemM.Match(&holder{List: []*int{&five}}) {
			t.Fatal("$elemMatch on typed slice failed")
		}
		if !elemM.Match(M{"List": A{&five, nilInt}}) || elemM.Match(M{"List": A{&five, nil}}) {
			t.Fatal("$elemMatch on []any typed nil failed")
		}
	})

	t.Run("array element vs whole-array custom op + $not + limit + error wrapping", func(t *testing.T) {
		// B2 + B3: implicit element matching vs whole slice operator
		m := mongory.MustCompile(M{
			"prices":     M{"$moneyGte": twd200},
			"all_prices": M{"$sumUnitsGte": 250},
			"blocked":    M{"$not": M{"$moneyGte": twd200}},
		}, opts)

		passRec := M{
			"prices":     []*testMoney{usd300, twd200},
			"all_prices": []*testMoney{twd100, twd200},
			"blocked":    twd100,
		}
		if !m.Match(passRec) {
			_, tr := m.Trace(passRec)
			t.Fatalf("expected implicit array + whole array + $not match:\n%s", tr)
		}
		if ok, tr := m.Trace(passRec); !ok {
			t.Fatalf("expected passing Trace:\n%s", tr)
		}

		// Invalid custom arg returns ErrInvalidCondition wrapping user error:
		_, err := mongory.Compile(M{"x": M{"$moneyGte": "bad"}}, opts)
		if !errors.Is(err, mongory.ErrInvalidCondition) || !errors.Is(err, errMoneyArg) {
			t.Fatalf("expected ErrInvalidCondition + errMoneyArg, got %v", err)
		}

		// Compile returning nil Predicate is rejected as invalid condition:
		_, errNilPred := mongory.Compile(M{"x": M{"$bad": 1}}, mongory.WithOperator("$bad", mongory.Operator{
			Compile: func(any) (mongory.Predicate, error) { return nil, nil },
		}))
		if !errors.Is(errNilPred, mongory.ErrInvalidCondition) {
			t.Fatalf("expected ErrInvalidCondition for nil predicate, got %v", errNilPred)
		}
	})
}

func TestAndOrAndB5LeafConflicts(t *testing.T) {
	t.Run("And merges compatible ops cleanly without mutating caller maps", func(t *testing.T) {
		baseStatus := M{"$ne": "removed"}
		base := M{"merchant_id": "m1", "status": baseStatus}
		extra := M{"status": M{"$in": A{"active", "pending"}}}
		nested := M{"$and": A{M{"$and": A{M{"status": M{"$ne": "removed"}}}}, M{"tier": "pro"}}}

		got := mongory.And(nil, base, M{}, extra, nested)

		// Base maps must remain unmutated!
		if _, leaked := baseStatus["$in"]; leaked {
			t.Fatal("And mutated caller's nested operator map")
		}
		if _, leaked := base["tier"]; leaked {
			t.Fatal("And mutated caller's base map")
		}

		m := mongory.MustCompile(got)
		if !m.Match(M{"merchant_id": "m1", "status": "active", "tier": "pro"}) {
			t.Fatalf("expected merged And query to match:\n%s", m.Explain())
		}
		if m.Match(M{"merchant_id": "m1", "status": "removed", "tier": "pro"}) {
			t.Fatal("removed record should not match")
		}
	})

	t.Run("And does not merge coupled $regex/$options or plain embedded docs across clauses", func(t *testing.T) {
		c1 := M{"name": M{"$regex": "^foo"}}
		c2 := M{"name": M{"$options": "i"}}
		gotRegex := mongory.And(c1, c2)
		if _, hasAnd := gotRegex["$and"]; !hasAnd {
			t.Fatalf("must not merge $regex and $options from separate And args: %#v", gotRegex)
		}

		sub1 := M{"addr": M{"city": "Taipei"}}
		sub2 := M{"addr": M{"zip": "100"}}
		gotSub := mongory.And(sub1, sub2)
		if _, hasAnd := gotSub["$and"]; !hasAnd {
			t.Fatalf("plain subdocs must preserve separate $and clauses for Mongo semantics: %#v", gotSub)
		}
		m := mongory.MustCompile(gotSub)
		if !m.Match(M{"addr": M{"city": "Taipei", "zip": "100"}}) {
			t.Fatal("merged subdoc $and failed in mongory")
		}
	})

	t.Run("defined bson.M-like maps interoperate cleanly with And/Or", func(t *testing.T) {
		type bsonM map[string]any
		type bsonA []any
		b1 := bsonM{"status": bsonM{"$ne": "removed"}}
		b2 := bsonM{"status": bsonM{"$in": bsonA{"active"}}}

		merged := mongory.And(b1, b2)
		var out bsonM = merged
		m := mongory.MustCompile(out)
		if !m.Match(M{"status": "active"}) || m.Match(M{"status": "removed"}) {
			t.Fatal("bsonM-compatible And output failed")
		}

		if len(mongory.And(nil, M{})) != 0 {
			t.Fatal("empty/identity And must return empty M{}")
		}
	})

	t.Run("Or Mongo semantics (nil skipped, explicit M{} absorbs, empty Or fails closed)", func(t *testing.T) {
		var unset M // nil map
		gotSkipped := mongory.Or(unset, M{"a": 1}, M{"a": 1}, M{"b": 2})
		m := mongory.MustCompile(gotSkipped)
		if m.Match(M{"c": 3}) || !m.Match(M{"b": 2}) {
			t.Fatalf("Or with omitted nil must not widen to match-all:\n%s", m.Explain())
		}
		if len(gotSkipped["$or"].(A)) != 2 {
			t.Fatalf("expected deduplicated 2-branch $or, got %#v", gotSkipped)
		}

		gotAbsorbed := mongory.Or(M{"a": 1}, M{"$or": A{M{"b": 2}, M{}}})
		if len(gotAbsorbed) != 0 {
			t.Fatalf("expected non-nil empty M{} to absorb Or to M{}, got %#v", gotAbsorbed)
		}

		gotEmpty := mongory.Or(nil, unset)
		emptyM := mongory.MustCompile(gotEmpty)
		if emptyM.Match(M{"a": 1}) {
			t.Fatal("empty Or() must fail closed (match nothing)")
		}
	})

	t.Run("B5 normalization preserves colliding dotted keys instead of overwriting", func(t *testing.T) {
		mIn := mongory.MustCompile(M{"a": M{"$in": A{1}}, "a.$in": A{2}})
		if mIn.Match(M{"a": 1}) || mIn.Match(M{"a": 2}) || !mIn.Match(M{"a": A{1, 2}}) {
			t.Fatalf("colliding $in conditions must both be enforced:\n%s", mIn.Explain())
		}

		mEq := mongory.MustCompile(M{"a": M{"b": 2}, "a.b": 1})
		if mEq.Match(M{"a": M{"b": 1}}) || mEq.Match(M{"a": M{"b": 2}}) {
			t.Fatalf("conflicting dotted field keys must both apply:\n%s", mEq.Explain())
		}

		mScalarVsObj := mongory.MustCompile(M{"a": 1, "a.b": 2})
		if mScalarVsObj.Match(M{"a": 1}) || mScalarVsObj.Match(M{"a": M{"b": 2}}) {
			t.Fatal("conflicting scalar vs nested field must both apply")
		}
	})
}

func TestFilterLimitEarlyStop(t *testing.T) {
	var evalCount atomic.Int64
	m := mongory.MustCompile(
		M{"v": M{"$trackGte": 2}},
		mongory.WithOperator("$trackGte", mongory.Operator{
			Compile: func(arg any) (mongory.Predicate, error) {
				minVal := arg.(int)
				return func(v any, exists bool) bool {
					evalCount.Add(1)
					n, ok := v.(int)
					return exists && ok && n >= minVal
				}, nil
			},
		}),
	)

	type item struct {
		V int `bson:"v"`
	}
	items := []item{{1}, {2}, {3}, {4}, {5}}

	if got := mongory.FilterLimit(items, m, 0); got != nil {
		t.Fatalf("limit 0 expected nil, got %v", got)
	}
	evalCount.Store(0)
	got2 := mongory.FilterLimit(items, m, 2)
	if !slices.Equal(got2, []item{{2}, {3}}) {
		t.Fatalf("limit 2 got %v, want [{2} {3}]", got2)
	}
	if evalCount.Load() != 3 {
		t.Fatalf("expected early stop after 3 evaluations, got %d", evalCount.Load())
	}

	gotAll := mongory.FilterLimit(items, m, -1)
	if !slices.Equal(gotAll, []item{{2}, {3}, {4}, {5}}) {
		t.Fatalf("negative limit got %v", gotAll)
	}

	ptrs := []*item{{1}, {2}, {3}}
	if gotPtr := mongory.FilterLimit(ptrs, m, 1); len(gotPtr) != 1 || gotPtr[0].V != 2 {
		t.Fatalf("pointer slice FilterLimit got %v", gotPtr)
	}
	if none := mongory.FilterLimit([]item{{1}}, m, 5); none != nil {
		t.Fatalf("expected nil when no elements match, got %v", none)
	}
}

func TestExtensionsAdditionalEdgesAndTrace(t *testing.T) {
	t.Run("converter registry map promotion >4 entries, interface override, map record regex/present/in", func(t *testing.T) {
		type c1 string
		type c2 string
		type c3 string
		type c4 string
		type c5 string

		opts := mongory.WithOptions(
			mongory.WithConverter(func(v c1) any { return "1:" + string(v) }),
			mongory.WithConverter(func(v c2) any { return "2:" + string(v) }),
			mongory.WithConverter(func(v c3) any { return "3:" + string(v) }),
			mongory.WithConverter(func(v c4) any { return "4:" + string(v) }),
			mongory.WithConverter(func(v c5) any { return "5:" + string(v) }),
			mongory.WithConverter(func(h hexer) any { return "old:" + h.Hex() }),
			mongory.WithConverter(func(h hexer) any { return "iface:" + h.Hex() }),
			mongory.WithConverter(func(ns nullableString) any {
				if !ns.Valid {
					return nil
				}
				return ns.String
			}),
		)

		inList := make(A, 20)
		inList[0] = "5:ok"
		for i := 1; i < 20; i++ {
			inList[i] = fmt.Sprintf("item-%d", i)
		}

		oid := testOID{'a', '1', 'b', '2'}
		m := mongory.MustCompile(M{
			"f1":   "1:a",
			"f5":   M{"$in": inList},
			"hex":  "iface:a1b2",
			"nick": M{"$regex": "^go", "$present": true},
		}, opts)

		rec := M{
			"f1":   c1("a"),
			"f5":   c5("ok"),
			"hex":  &oid,
			"nick": nullableString{String: "gopher", Valid: true},
		}
		if !m.Match(rec) {
			t.Fatalf("expected converter >4 types + iface + map record match")
		}
		if ok, tr := m.Trace(rec); !ok || tr == "" {
			t.Fatalf("expected passing Trace on converted map record:\n%s", tr)
		}
		badRec := M{
			"f1":   c1("a"),
			"f5":   c5("ok"),
			"hex":  (*testOID)(nil),
			"nick": nullableString{Valid: false},
		}
		if m.Match(badRec) {
			t.Fatal("nil *testOID and invalid nullableString must not match")
		}
	})

	t.Run("Trace parity on inline map, bson.D, slice index, resolver fallback, and cyclic struct", func(t *testing.T) {
		type traceRec struct {
			Tags   []string
			Prices []*testMoney
			Extra  map[string]any `bson:",inline"`
		}
		type cycStruct struct {
			Name string
			Next *cycStruct
		}
		cs := &cycStruct{Name: "loop"}
		cs.Next = cs

		opts := mongory.WithOptions(
			mongory.WithConverter(func(m *testMoney) any { return m.Units }),
			mongory.WithFieldResolver(func(r *traceRec, k string) (any, bool) {
				if k == "virt" {
					return "yes", true
				}
				return nil, false
			}),
			mongory.WithOperator("$everyGt0", mongory.Operator{
				MatchElements: true,
				Compile: func(any) (mongory.Predicate, error) {
					return func(v any, exists bool) bool {
						m, ok := v.(*testMoney)
						return exists && ok && m != nil && m.Units > 0
					}, nil
				},
			}),
		)

		m := mongory.MustCompile(M{
			"virt":   "yes",
			"dyn":    10,
			"Tags.0": "a",
			"Tags":   M{"$size": 2, "$every": M{"$regex": "^[ab]$"}},
			"Prices": M{"$every": M{"$everyGt0": true}},
		}, opts)

		rec := &traceRec{
			Tags:   []string{"a", "b"},
			Prices: []*testMoney{{Units: 5}, {Units: 10}},
			Extra:  M{"dyn": 10},
		}
		if !m.Match(rec) {
			t.Fatal("expected traceRec match")
		}
		if ok, tr := m.Trace(rec); !ok {
			t.Fatalf("expected passing Trace:\n%s", tr)
		}

		// bson.D record + negative slice index + resolver missing key:
		dRec := bsonD{{"k", "v"}, {"arr", []string{"x", "y"}}}
		md := mongory.MustCompile(M{"k": "v", "arr.-1": "y", "missing": M{"$exists": false}})
		if !md.Match(dRec) {
			t.Fatal("expected bson.D match")
		}
		if ok, tr := md.Trace(dRec); !ok {
			t.Fatalf("expected bson.D Trace:\n%s", tr)
		}

		// Cyclic struct must not stack overflow in Trace:
		mc := mongory.MustCompile(M{"Name": "loop", "Next.Name": "loop"})
		if ok, tr := mc.Trace(cs); !ok || tr == "" {
			t.Fatalf("expected safe Trace on cyclic struct, ok=%v", ok)
		}
	})

	t.Run("WithField override precedence, Combinator edges, and FilterLimit concurrency", func(t *testing.T) {
		type item struct {
			V int
		}
		m := mongory.MustCompile(
			M{"v": 20},
			mongory.WithField("v", func(i item) any { return i.V + 1 }),
			mongory.WithField("v", func(i item) any { return i.V * 2 }),
		)
		if !m.Match(item{V: 10}) || m.Match(item{V: 19}) {
			t.Fatal("later WithField must override earlier registration")
		}

		// Multi-key $and flattening + []M typed slice inside $and/$or + conflicting pure ops + malformed $and:
		c1 := mongory.And(
			M{"$and": []M{{"a": 1}}, "x": 10},
			M{"status": M{"$gt": 1}},
			M{"status": M{"$gt": 2}},
		)
		compiledC1 := mongory.MustCompile(c1)
		if !compiledC1.Match(M{"a": 1, "x": 10, "status": 3}) || compiledC1.Match(M{"a": 1, "x": 10, "status": 2}) {
			t.Fatalf("conflicting $gt in And failed: %#v", c1)
		}

		// Malformed $and value preserved for Compile error:
		badAnd := mongory.And(M{"$and": "not-slice"}, M{"a": 1})
		if _, err := mongory.Compile(badAnd); !errors.Is(err, mongory.ErrInvalidCondition) {
			t.Fatalf("expected ErrInvalidCondition for malformed $and preserved by And, got %v", err)
		}

		// Malformed $or preserved for Compile error:
		badOr := mongory.Or(M{"$or": "not-slice"}, M{"b": 2})
		if _, err := mongory.Compile(badOr); !errors.Is(err, mongory.ErrInvalidCondition) {
			t.Fatalf("expected ErrInvalidCondition for malformed $or preserved by Or, got %v", err)
		}

		// Concurrent Match/Trace across goroutines with extensions:
		opts := mongory.WithOptions(
			mongory.WithConverter(func(o testOID) any { return o.Hex() }),
			mongory.WithField("len", func(p *promoCondModel) any { return len(p.Tags) }),
			mongory.WithOperator("$hasOid", mongory.Operator{
				Compile: func(arg any) (mongory.Predicate, error) {
					want := arg.(string)
					return func(v any, exists bool) bool {
						o, ok := v.(testOID)
						return exists && ok && o.Hex() == want
					}, nil
				},
			}),
		)
		cm := mongory.MustCompile(M{"len": M{"$gte": 1}, "promotion_id": M{"$hasOid": "1234"}}, opts)
		rec := promoCondModel{PromotionID: testOID{'1', '2', '3', '4'}, Tags: []testOID{{'a', 'b', 'c', 'd'}}}

		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 200 {
					if !cm.Match(&rec) {
						t.Error("concurrent match failed")
						return
					}
					if ok, _ := cm.Trace(&rec); !ok {
						t.Error("concurrent trace failed")
						return
					}
				}
			}()
		}
		wg.Wait()
	})
}

func TestConverterContainersAndExtFields(t *testing.T) {
	oid1 := testOID{'o', 'i', 'd', '1'}
	oid2 := testOID{'o', 'i', 'd', '2'}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	opts := mongory.WithOptions(
		mongory.WithConverter(func(o testOID) any { return o.Hex() }),
		mongory.WithConverter(func(m *testMoney) any { return m.Units }),
	)

	t.Run("Explain, Trace, and nested map/slice/struct equality with converters", func(t *testing.T) {
		type richDoc struct {
			ID     testOID     `bson:"id"`
			Flag   bool        `bson:"flag"`
			Name   string      `bson:"name"`
			Count  int         `bson:"count"`
			UCount uint64      `bson:"ucount"`
			Score  float64     `bson:"score"`
			At     time.Time   `bson:"at"`
			Num    json.Number `bson:"num"`
			List   []testOID   `bson:"list"`
			Inner  *testMoney  `bson:"inner"`
		}
		docVal := richDoc{
			ID:     oid1,
			Flag:   true,
			Name:   "go",
			Count:  7,
			UCount: 9,
			Score:  1.5,
			At:     now,
			Num:    json.Number("42"),
			List:   []testOID{oid1, oid2},
			Inner:  &testMoney{Units: 50},
		}

		// Condition compares nested whole struct via $eq, whole slice via $ne, map literal, and $in fallback:
		largeTargets := make(A, 18)
		largeTargets[0] = M{"id": "oid1"}
		largeTargets[1] = 100
		largeTargets[2] = now
		largeTargets[3] = [4]byte{'a', 'b', 'c', 'd'}
		for i := 4; i < 18; i++ {
			largeTargets[i] = fmt.Sprintf("pad-%d", i)
		}

		m := mongory.MustCompile(M{
			"doc": M{"$eq": M{
				"id":     "oid1",
				"flag":   true,
				"name":   "go",
				"count":  7,
				"ucount": 9,
				"score":  1.5,
				"at":     now,
				"num":    42,
				"list":   A{"oid1", "oid2"},
				"inner":  50,
			}},
			"arr_eq": M{"$eq": A{"oid1", "oid2"}, "$ne": A{"oid9"}},
			"in_mix": M{"$in": largeTargets},
			"map_in": M{"$in": A{M{"k": oid1}}},
			"num_in": M{"$in": largeTargets},
		}, opts)

		if ex := m.Explain(); !strings.Contains(ex, "Eq(") || !strings.Contains(ex, "Ne(") || !strings.Contains(ex, "In(") {
			t.Fatalf("unexpected Explain with converter wrappers:\n%s", ex)
		}

		mapRec := M{
			"doc":    docVal,
			"arr_eq": A{oid1, oid2},
			"in_mix": A{M{"id": oid1}},
			"map_in": M{"k": "oid1"},
			"num_in": json.Number("100"),
		}
		if !m.Match(mapRec) {
			_, tr := m.Trace(mapRec)
			t.Fatalf("expected converter container match:\n%s", tr)
		}
		if ok, tr := m.Trace(mapRec); !ok || tr == "" {
			t.Fatalf("expected passing Trace on converter container:\n%s", tr)
		}

		// Large $in with nil, float, bool, string, [4]byte, time.Time via map and reflect:
		mInScalar := mongory.MustCompile(M{
			"tags":  M{"$in": largeTargets},
			"arr":   M{"$in": largeTargets},
			"small": M{"$in": A{oid1, M{"id": oid2}}},
		}, opts)
		recIn := M{
			"tags":  A{nil, 100, oid1},
			"arr":   [4]byte{'a', 'b', 'c', 'd'},
			"small": bsonD{{"id", oid2}},
		}
		if !mInScalar.Match(recIn) {
			t.Fatalf("expected inConvMatcher map/reflect array match")
		}
		if ok, tr := mInScalar.Trace(recIn); !ok || tr == "" {
			t.Fatalf("Trace failed on inConvMatcher:\n%s", tr)
		}

		// bson.D equality under eqConvMatcher + json.Number on extLiteralMatcher:
		mBsonD := mongory.MustCompile(M{
			"d":   M{"$eq": M{"id": "oid1"}},
			"jn":  42,
			"bad": M{"$ne": "oid2"},
		}, opts)
		recBsonD := M{
			"d":   bsonD{{"id", oid1}},
			"jn":  json.Number("42"),
			"bad": M{"id": "oid1"},
		}
		if !mBsonD.Match(recBsonD) {
			t.Fatal("expected eqConvMatcher bson.D and json.Number match")
		}
	})

	t.Run("extFieldMatcher raw custom op on bson.D, slice index, and inline map trace", func(t *testing.T) {
		var nilInt *int
		five := 5
		rawOpts := mongory.WithOptions(
			mongory.WithOperator("$isTypedNilInt", mongory.Operator{
				Compile: func(arg any) (mongory.Predicate, error) {
					want := arg.(bool)
					return func(v any, exists bool) bool {
						if !exists {
							return !want
						}
						p, ok := v.(*int)
						return (ok && p == nil) == want
					}, nil
				},
			}),
			mongory.WithFieldResolver(func(b namedAttrBag, key string) (any, bool) {
				v, ok := b[key]
				return v, ok
			}),
		)

		mRaw := mongory.MustCompile(M{
			"item":    M{"$isTypedNilInt": true},
			"arr.0":   M{"$isTypedNilInt": false},
			"arr.1":   M{"$isTypedNilInt": true},
			"missing": M{"$isTypedNilInt": false},
			"arr.9":   M{"$isTypedNilInt": false},
		}, rawOpts)

		dRec := bsonD{{"item", nilInt}, {"arr", A{&five, nilInt}}}
		if !mRaw.Match(dRec) {
			_, tr := mRaw.Trace(dRec)
			t.Fatalf("expected raw extFieldMatcher match on bson.D & slice index:\n%s", tr)
		}
		if ok, tr := mRaw.Trace(dRec); !ok {
			t.Fatalf("Trace failed on extFieldMatcher bson.D:\n%s", tr)
		}

		// Struct with inline map + getter Trace parity:
		type inlineHolder struct {
			Extra map[string]any `bson:",inline"`
		}
		mInline := mongory.MustCompile(M{"x": M{"$isTypedNilInt": true}}, mongory.WithOptions(rawOpts, mongory.WithFieldResolver(func(h *inlineHolder, key string) (any, bool) {
			return nil, false
		})))
		hRec := &inlineHolder{Extra: M{"x": nilInt}}
		if !mInline.Match(hRec) {
			t.Fatal("expected inline map match via extFieldMatcher")
		}
		if ok, tr := mInline.Trace(hRec); !ok {
			t.Fatalf("Trace failed on extFieldMatcher inline map:\n%s", tr)
		}

		// Trigger extLiteralMatcher arrayRecord Trace:
		mArrTrace := mongory.MustCompile(M{"arr": M{"$isTypedNilInt": true}}, mongory.WithOptions(rawOpts))
		if ok, tr := mArrTrace.Trace(M{"arr": A{nilInt}}); ok || tr == "" {
			// MatchElements is false so array passed whole; both Match and Trace agree false
			if ok != mArrTrace.Match(M{"arr": A{nilInt}}) {
				t.Fatalf("Match vs Trace mismatch:\n%s", tr)
			}
		}
	})
}
