package mongory_test

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mongoryhq/mongory-go"
)

type (
	M = mongory.M
	A = mongory.A
)

type Address struct {
	City string `bson:"city"`
}

type Base struct {
	ID int `bson:"_id"`
}

type Meta struct {
	Source string `bson:"source"`
}

type person struct {
	Base        `bson:",inline"`
	Meta                       // embedded without inline: "Meta" subdocument (exact field name)
	Name        string         `mongory:"full_name" bson:"name" json:"display_name"`
	Age         uint8          `bson:"age,omitempty" json:"years"`
	Tags        []string       `bson:"tags"`
	Address     *Address       `bson:"address"`
	Secret      string         `bson:"-"`
	Computed    string         `bson:"-" json:"computed"`
	BsonEmpty   string         `bson:",omitempty" json:"from_json"`
	MongoryDash string         `mongory:"-"`
	Nickname    string         `json:"nick,omitempty"`
	UserCode    string         `json:",omitempty"`
	ProductID   string         `clone:"true"`
	PrivateKey  string         `json:"-"`
	PublicDash  string         `json:"-,"`
	JSONSecret  string         `json:"-" bson:"json_secret"`
	Extra       map[string]any `bson:",inline"`
	hidden      string
}

type role string

type wrapper struct {
	*Base `bson:",inline"`
	Role  role
	Extra map[string]any `bson:",inline"`
}

type event struct {
	At    time.Time
	Count json.Number
	Raw   json.Number
}

// Driver look-alikes: matched by shape, so no mongo-driver dependency.
type (
	bsonE struct {
		Key   string
		Value any
	}
	bsonD     []bsonE
	objectID  [12]byte
	dateTime  int64
	bsonRegex struct {
		Pattern string
		Options string
	}
)

func (d dateTime) Time() time.Time { return time.UnixMilli(int64(d)).UTC() }

func TestMatcherMatch(t *testing.T) {
	alice := person{
		Base:        Base{ID: 7},
		Meta:        Meta{Source: "web"},
		Name:        "Alice",
		Age:         30,
		Tags:        []string{"go", "ruby"},
		Address:     &Address{City: "Taipei"},
		Secret:      "s",
		Computed:    "c",
		BsonEmpty:   "bj",
		MongoryDash: "md",
		Nickname:    "al",
		UserCode:    "U-1",
		ProductID:   "P-9",
		PrivateKey:  "priv",
		PublicDash:  "dash",
		JSONSecret:  "kept",
		Extra:       M{"plan": "pro", "full_name": "shadowed"},
		hidden:      "h",
	}
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	oid := objectID{1, 2, 3}

	tests := []struct {
		name      string
		condition any
		record    any
		want      bool
	}{
		{"empty condition matches scalar", M{}, "anything", true},
		{"nil condition matches nil", nil, nil, true},
		{"plain field", M{"name": "John"}, M{"name": "John"}, true},
		{"plain field mismatch", M{"name": "John"}, M{"name": "john"}, false},
		{"field on scalar record", M{"name": "John"}, "John", false},
		{"int condition matches float record", M{"age": 30}, M{"age": 30.0}, true},
		{"float condition rejects fractional", M{"age": 30.5}, M{"age": 30}, false},
		{"typed ints are widened", M{"n": int8(3)}, M{"n": uint64(3)}, true},
		{"json.Number record", M{"n": 3}, M{"n": json.Number("3")}, true},
		{"nested object", M{"a": M{"b": M{"c": 1}}}, M{"a": M{"b": M{"c": 1, "d": 2}}}, true},
		{"nested nil parent", M{"a": M{"b": 1}}, M{"a": nil}, false},
		{"dot path", M{"a.b.c": 1}, M{"a": M{"b": M{"c": 1}}}, true},
		{"dot paths deep-merge", M{"a.b": 1, "a.c": 2}, M{"a": M{"b": 1, "c": 2}}, true},
		{"dot paths deep-merge mismatch", M{"a.b": 1, "a.c": 2}, M{"a": M{"b": 1, "c": 3}}, false},
		{"escaped dot is literal", M{`a\.b`: 1}, M{"a.b": 1}, true},
		{"escaped dot does not split", M{`a\.b`: 1}, M{"a": M{"b": 1}}, false},
		{"null matches missing", M{"a": nil}, M{}, true},
		{"null matches explicit nil", M{"a": nil}, M{"a": nil}, true},
		{"null rejects value", M{"a": nil}, M{"a": 0}, false},
		{"$eq null rejects missing", M{"a": M{"$eq": nil}}, M{}, false},
		{"array index", M{"tags.0": "go"}, M{"tags": A{"go", "c"}}, true},
		{"negative array index", M{"tags.-1": "c"}, M{"tags": A{"go", "c"}}, true},
		{"index out of range", M{"tags.5": "c"}, M{"tags": A{"go", "c"}}, false},
		{"index key on object", M{"tags.0": "go"}, M{"tags": M{"0": "go"}}, true},
		{"leading zero key is element field", M{"tags.01": "c"}, M{"tags": A{M{"01": "c"}}}, true},
		{"scalar in array", M{"tags": "ruby"}, M{"tags": A{"go", "ruby"}}, true},
		{"scalar not in array", M{"tags": "c"}, M{"tags": A{"go", "ruby"}}, false},
		{"whole array equality", M{"tags": A{"go", "ruby"}}, M{"tags": A{"go", "ruby"}}, true},
		{"array order matters", M{"tags": A{"ruby", "go"}}, M{"tags": A{"go", "ruby"}}, false},
		{"nested array element", M{"tags": A{"go"}}, M{"tags": A{A{"go"}, "c"}}, true},
		{"implicit elemMatch on objects", M{"people": M{"name": "Billy", "age": 18}},
			M{"people": A{M{"name": "Billy", "age": 18}, M{"name": "Bob"}}}, true},
		{"implicit elemMatch needs one element", M{"people": M{"name": "Billy", "age": 18}},
			M{"people": A{M{"name": "Billy", "age": 1}, M{"name": "Bob", "age": 18}}}, false},
		{"explicit $elemMatch", M{"xs": M{"$elemMatch": M{"a": 1}}}, M{"xs": A{M{"a": 2}, M{"a": 1}}}, true},
		{"$elemMatch on non-array", M{"xs": M{"$elemMatch": M{"a": 1}}}, M{"xs": M{"a": 1}}, false},
		{"object equality ignores key order", M{"$eq": M{"a": 1, "b": 2}}, M{"b": 2, "a": 1}, true},
		{"object equality needs same keys", M{"o": M{"$eq": M{"a": 1}}}, M{"o": M{"a": 1, "b": 2}}, false},
		{"time equality", M{"at": createdAt}, M{"at": createdAt.In(time.FixedZone("x", 3600))}, true},
		{"typed map record", M{"n": 1}, map[string]int{"n": 1}, true},
		{"typed slice record", M{"0": "a"}, []string{"a", "b"}, true},
		{"struct mongory tag wins", M{"full_name": "Alice", "name": nil, "display_name": nil}, alice, true},
		{"struct bson tag wins over json", M{"age": 30, "years": nil, "address.city": "Taipei"}, alice, true},
		{"struct pointer", M{"age": 30}, &alice, true},
		{"struct typed slice", M{"tags": "ruby"}, alice, true},
		{"struct inline embedded", M{"_id": 7}, alice, true},
		{"struct embedded uses exact field name subdocument", M{"Meta.source": "web"}, alice, true},
		{"struct embedded not lowercased", M{"meta.source": "web"}, alice, false},
		{"struct embedded lowercased key absent", M{"meta": nil}, alice, true},
		{"struct embedded not flattened", M{"source": nil}, alice, true},
		{"struct json tag resolves field", M{"nick": "al"}, alice, true},
		{"struct json tag ignores Go field name", M{"Nickname": nil}, alice, true},
		{"struct json omitempty empty name uses exact field name", M{"UserCode": "U-1", "usercode": nil}, alice, true},
		{"struct untagged exact field name", M{"ProductID": "P-9"}, alice, true},
		{"struct untagged not lowercased", M{"productid": nil, "product_id": nil}, alice, true},
		{"struct json dash tag uses exact field name", M{"PrivateKey": "priv", "privatekey": nil}, alice, true},
		{"struct json escaped dash matches literal key", M{"-": "dash", "PublicDash": nil}, alice, true},
		{"struct bson tag overrides json dash", M{"json_secret": "kept", "JSONSecret": nil}, alice, true},
		{"struct bson dash tag uses exact field name", M{"Secret": "s", "secret": nil}, alice, true},
		{"struct bson dash falls back to json", M{"computed": "c", "Computed": nil}, alice, true},
		{"struct bson empty name falls back to json", M{"from_json": "bj", "BsonEmpty": nil}, alice, true},
		{"struct mongory dash falls back to field name", M{"MongoryDash": "md"}, alice, true},
		{"struct unexported skipped", M{"hidden": nil}, alice, true},
		{"struct unexported embedded inline promotes exported fields", M{"code": "C1", "hidden": nil}, fuzzRecord{fuzzInline: &fuzzInline{Code: "C1"}}, true},
		{"struct unexported embedded without inline stays hidden", M{"Code": nil}, struct{ *fuzzInline }{&fuzzInline{Code: "C1"}}, true},
		{"struct inline map", M{"plan": "pro"}, alice, true},
		{"struct inline map never overrides fields", M{"full_name": "Alice"}, alice, true},
		{"struct nil inline pointer", M{"_id": nil, "Role": "admin"}, wrapper{Role: "admin"}, true},
		{"struct nil inline pointer falls back to inline map", M{"_id": 99}, wrapper{Role: "admin", Extra: M{"_id": 99}}, true},
		{"struct time field equality", M{"At": createdAt}, event{At: createdAt}, true},
		{"struct time field not treated as subdocument", M{"At.year": M{"$exists": true}}, event{At: createdAt}, false},
		{"struct json.Number equality and regex", M{"Count": 42, "Raw": M{"$regex": "^abc"}}, event{Count: json.Number("42"), Raw: json.Number("abcdef")}, true},
		{"empty struct not present", M{"$present": false}, struct{}{}, true},
		{"struct slice $eq/$ne element match", M{"tags": M{"$eq": "ruby"}, "tags.$ne": "python"}, alice, true},
		{"struct as condition", Meta{Source: "web"}, M{"source": "web"}, true},
		{"struct as condition with json and exact keys", struct {
			ProductID string
			Nick      string `json:"nick"`
		}{ProductID: "p1", Nick: "al"}, M{"ProductID": "p1", "nick": "al"}, true},
		{"nil pointer record", M{"a": 1}, (*person)(nil), false},
		{"bson.D condition", bsonD{{"age", 30}, {"name", "Al"}}, M{"age": 30, "name": "Al"}, true},
		{"bson.D condition mismatch", bsonD{{"age", 30}}, M{"age": 31}, false},
		{"nested bson.D condition", M{"xs": bsonD{{"$elemMatch", bsonD{{"a", 1}}}}}, M{"xs": A{M{"a": 1}}}, true},
		{"bson.D record", M{"a.b": 1}, bsonD{{"a", bsonD{{"b", 1}}}}, true},
		{"ObjectID equality", M{"_id": oid}, M{"_id": oid}, true},
		{"ObjectID inequality", M{"_id": oid}, M{"_id": objectID{9}}, false},
		{"ObjectID is not an array", M{"_id.0": 1}, M{"_id": oid}, false},
		{"DateTime equals time.Time", M{"at": createdAt}, M{"at": dateTime(createdAt.UnixMilli())}, true},
		{"$gt int vs float exact", M{"n": M{"$gt": 9007199254740992.0}}, M{"n": 9007199254740993}, true},
		{"$lt float vs int exact", M{"n": M{"$lt": 9007199254740993}}, M{"n": 9007199254740992.0}, true},
		{"$lt int vs fractional float", M{"n": M{"$lt": 2.5}}, M{"n": 2}, true},
		{"$gte float vs equal int", M{"n": M{"$gte": 2.0}}, M{"n": 2}, true},
		{"$gt NaN never matches", M{"n": M{"$gt": 1}}, M{"n": math.NaN()}, false},
		{"$lte NaN never matches", M{"n": M{"$lte": math.NaN()}}, M{"n": 1}, false},
		{"$gt bool incomparable", M{"b": M{"$gt": false}}, M{"b": true}, false},
		{"$gt time", M{"at": M{"$gt": createdAt}}, M{"at": createdAt.Add(time.Second)}, true},
		{"$lt DateTime vs time", M{"at": M{"$lt": createdAt}}, M{"at": dateTime(createdAt.UnixMilli() - 1)}, true},
		{"$gt array implicit elemMatch", M{"xs": M{"$gt": 5}}, M{"xs": []int{1, 9}}, true},
		{"$ne missing", M{"a": M{"$ne": 1}}, M{}, true},
		{"$in typed slice condition", M{"n": M{"$in": []int{1, 2}}}, M{"n": 2.0}, true},
		{"$in missing never matches", M{"n": M{"$in": A{nil}}}, M{}, false},
		{"$nin missing matches", M{"n": M{"$nin": A{1}}}, M{}, true},
		{"$exists false on missing", M{"a": M{"$exists": false}}, M{}, true},
		{"$present zero is present", M{"a": M{"$present": true}}, M{"a": 0}, true},
		{"$present empty struct slice", M{"tags": M{"$present": false}}, person{Tags: []string{}}, true},
		{"$not missing field", M{"a": M{"$not": M{"$gt": 1}}}, M{}, true},
		{"$and of $or", M{"$and": A{M{"$or": A{M{"a": 1}, M{"b": 1}}}, M{"c": 1}}}, M{"b": 1, "c": 1}, true},
		{"$every typed slice", M{"xs": M{"$every": M{"$gte": 1}}}, M{"xs": []int{1, 2}}, true},
		{"$size typed slice", M{"xs": M{"$size": 2}}, M{"xs": []string{"a", "b"}}, true},
		{"operator in dot path", M{"xs.$size.$gte": 2}, M{"xs": A{1, 2, 3}}, true},
		{"$regex with $options", M{"s": M{"$regex": "^ab", "$options": "i"}}, M{"s": "ABC"}, true},
		{"$regex without $options is case sensitive", M{"s": M{"$regex": "^ab"}}, M{"s": "ABC"}, false},
		{"$regex $options on array implicit elemMatch", M{"xs": M{"$regex": "^ab", "$options": "i"}}, M{"xs": A{"x", "ABx"}}, true},
		{"*regexp.Regexp literal", M{"s": regexp.MustCompile("b+")}, M{"s": "abbc"}, true},
		{"*regexp.Regexp literal on array", M{"xs": regexp.MustCompile("^b")}, M{"xs": A{"a", "bc"}}, true},
		{"BSON regex literal", M{"s": bsonRegex{Pattern: "^ab", Options: "i"}}, M{"s": "AB"}, true},
		{"$regex ignores non-strings", M{"s": M{"$regex": "1"}}, M{"s": 1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := mongory.Compile(tt.condition)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got := m.Match(tt.record); got != tt.want {
				t.Errorf("Match(%#v) = %v, want %v", tt.record, got, tt.want)
			}
		})
	}
}

func TestCompileInvalidCondition(t *testing.T) {
	_, err := mongory.Compile(M{"xs": M{"$elemMatch": 1}})
	if !errors.Is(err, mongory.ErrInvalidCondition) {
		t.Fatalf("err = %v, want mongory.ErrInvalidCondition", err)
	}
}

func TestMustCompilePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustCompile did not panic on invalid condition")
		}
	}()
	mongory.MustCompile(M{"$unknown": 1})
}

func TestOneShotMatch(t *testing.T) {
	ok, err := mongory.Match(M{"name": "Ann"}, M{"name": "Ann"})
	if err != nil || !ok {
		t.Fatalf("Match = (%v, %v), want (true, nil)", ok, err)
	}
	if _, err := mongory.Match(M{"$bad": 1}, M{}); !errors.Is(err, mongory.ErrInvalidCondition) {
		t.Fatalf("err = %v, want ErrInvalidCondition", err)
	}
}

func TestMatcherDoesNotMutateCondition(t *testing.T) {
	cond := M{"a.b": M{"c.d": 1}}
	if _, err := mongory.Compile(cond); err != nil {
		t.Fatal(err)
	}
	if _, ok := cond["a.b"].(M)["c.d"]; !ok {
		t.Fatalf("condition mutated: %#v", cond)
	}
}

func TestSliceHelpers(t *testing.T) {
	m := mongory.MustCompile(M{"age": M{"$gte": 18}})

	t.Run("Filter preserves defined slice type and order", func(t *testing.T) {
		type Docs []M
		records := Docs{{"age": 20}, {"age": 10}, {"age": 30}}
		got := mongory.Filter(records, m)
		if len(got) != 2 || got[0]["age"] != 20 || got[1]["age"] != 30 {
			t.Fatalf("Filter = %v", got)
		}
		if len(records) != 3 || records[1]["age"] != 10 {
			t.Fatalf("input modified: %v", records)
		}
	})

	t.Run("Find Any and Count", func(t *testing.T) {
		users := []*person{{Name: "Kid", Age: 10}, {Name: "Bob", Age: 25}, nil, {Name: "Cid", Age: 40}}
		first, ok := mongory.Find(users, m)
		if !ok || first == nil || first.Name != "Bob" {
			t.Fatalf("Find = (%v, %v), want Bob", first, ok)
		}
		if !mongory.Any(users, m) {
			t.Fatal("Any = false, want true")
		}
		if got := mongory.Count(users, m); got != 2 {
			t.Fatalf("Count = %d, want 2", got)
		}
	})

	t.Run("empty and no match", func(t *testing.T) {
		if got := mongory.Filter([]person{{Age: 1}}, m); got != nil {
			t.Fatalf("Filter = %v, want nil", got)
		}
		if got := mongory.Filter[[]M](nil, m); got != nil {
			t.Fatalf("Filter(nil) = %v, want nil", got)
		}
		if first, ok := mongory.Find([]person{{Age: 1}}, m); ok || first.Age != 0 {
			t.Fatalf("Find = (%v, %v), want zero/false", first, ok)
		}
		if mongory.Any([]person{{Age: 1}}, m) {
			t.Fatal("Any = true, want false")
		}
		if got := mongory.Count([]person{{Age: 1}}, m); got != 0 {
			t.Fatalf("Count = %d, want 0", got)
		}
	})
}

func TestExplainAndTrace(t *testing.T) {
	m := mongory.MustCompile(M{
		"$or": A{
			M{"role": "admin"},
			M{"tags": M{"$elemMatch": M{"$regex": "^go"}}},
		},
		"active": true,
	})

	explain := m.Explain()
	if m.String() != explain {
		t.Fatalf("String() != Explain()")
	}
	for _, want := range []string{"And", `Field("active")`, "Eq(true)", "Or", "ElemMatch", "Regex(/^go/)"} {
		if !strings.Contains(explain, want) {
			t.Fatalf("Explain missing %q:\n%s", want, explain)
		}
	}

	ok, trace := m.Trace(M{"active": false, "role": "admin", "tags": A{"go"}})
	if ok {
		t.Fatal("Trace matched unexpectedly")
	}
	if !strings.Contains(trace, "[FAIL] And") || !strings.Contains(trace, "[SKIP]") {
		t.Fatalf("expected short-circuit trace:\n%s", trace)
	}

	ok, trace = m.Trace(M{"active": true, "role": "user", "tags": A{"rust", "golang"}})
	if !ok {
		t.Fatal("Trace did not match")
	}
	if !strings.Contains(trace, "[PASS] Or") || !strings.Contains(trace, "[PASS] [1]") {
		t.Fatalf("expected ElemMatch element trace:\n%s", trace)
	}

	fullM := mongory.MustCompile(M{
		"$and": A{
			M{"role": M{"$ne": "guest", "$not": "banned"}},
			M{"scores": M{"$every": M{"$gte": 1}, "$size": 2}},
			M{"tags": M{"$in": A{"go"}, "$nin": A{"bad"}}},
			M{"nick": M{"$exists": true, "$present": true}},
		},
	})
	fullExplain := fullM.Explain()
	for _, want := range []string{"Ne(", "Not", "Every", "Size", "In(", "Nin(", "Exists(true)", "Present(true)"} {
		if !strings.Contains(fullExplain, want) {
			t.Fatalf("fullExplain missing %q:\n%s", want, fullExplain)
		}
	}
	if ok, tr := fullM.Trace(M{"role": "user", "scores": A{2, 3}, "tags": A{"go"}, "nick": "al"}); !ok || tr == "" {
		t.Fatalf("fullM Trace failed: ok=%v\n%s", ok, tr)
	}
	if s := mongory.MustCompile(M{}).Explain(); !strings.Contains(s, "And (empty)") {
		t.Fatalf("empty condition Explain = %q", s)
	}
	if s := mongory.MustCompile(M{"$or": A{}}).Explain(); !strings.Contains(s, "Or (empty)") {
		t.Fatalf("empty $or Explain = %q", s)
	}
}

func TestMatcherConcurrentSafety(t *testing.T) {
	m := mongory.MustCompile(M{
		"Tags":   "go",
		"Scores": M{"$gt": 5, "$lt": 8},
		"Note":   M{"$regex": "^ok", "$options": "i"},
	})

	type structDoc struct {
		Tags   []string
		Scores []int
		Note   string
	}
	type structDocAlt struct {
		Remark string   `json:"Note"`
		Labels []string `json:"Tags"`
		Points []int    `json:"Scores"`
	}

	const workers = 8
	const iters = 300
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				doc := M{"Tags": A{"ruby", "go"}, "Scores": A{3, 6, 9}, "Note": "OK-1"}
				s1 := structDoc{Tags: []string{"ruby", "go"}, Scores: []int{3, 6, 9}, Note: "OK-1"}
				s2 := structDocAlt{Labels: []string{"ruby", "go"}, Points: []int{3, 6, 9}, Remark: "OK-1"}
				if !m.Match(doc) || !m.Match(&s1) || !m.Match(s2) {
					errs <- errors.New("expected match")
					return
				}
				if ok, trace := m.Trace(doc); !ok || trace == "" {
					errs <- errors.New("expected passing trace")
					return
				}
				if m.Explain() == "" {
					errs <- errors.New("expected explain output")
					return
				}
				if m.Match(M{"Tags": "py", "Scores": 2, "Note": "no"}) || m.Match(&structDoc{Note: "no"}) {
					errs <- errors.New("unexpected match")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestTypedNilSlices(t *testing.T) {
	type customIDs []int
	type customAny []any
	type inFilter struct {
		In []string `mongory:"$in"`
	}

	var nilStrings []string
	var nilCustom customIDs
	var nilCustomAny customAny
	var nilMaps []M
	var nilSlicePtr *[]string
	var nilBsonD bsonD

	t.Run("typed nil slices act as empty in array operators", func(t *testing.T) {
		inNil := mongory.MustCompile(M{"id": M{"$in": nilStrings}})
		if inNil.Match(M{"id": "a"}) || inNil.Match(M{"id": nil}) || inNil.Match(M{}) {
			t.Fatal("$in with typed nil slice should never match")
		}

		ninNil := mongory.MustCompile(M{"id": M{"$nin": nilStrings}})
		if !ninNil.Match(M{"id": "a"}) || !ninNil.Match(M{"id": nil}) || !ninNil.Match(M{}) {
			t.Fatal("$nin with typed nil slice should match existing documents")
		}
		if ninNil.Match(nil) {
			t.Fatal("$nin on field should not match nil root record")
		}

		if m := mongory.MustCompile(M{"id": M{"$in": nilCustom}}); m.Match(M{"id": 1}) {
			t.Fatal("custom typed nil slice in $in should never match")
		}
		if m := mongory.MustCompile(M{"id": M{"$in": nilCustomAny}}); m.Match(M{"id": 1}) {
			t.Fatal("named []any nil slice in $in should never match")
		}
		if m := mongory.MustCompile(M{"id.$in": nilStrings}); m.Match(M{"id": "a"}) {
			t.Fatal("dot-path $in with typed nil slice should never match")
		}
		if m := mongory.MustCompile(M{"id.$nin": nilStrings}); !m.Match(M{"id": "a"}) {
			t.Fatal("dot-path $nin with typed nil slice should match")
		}
		if m := mongory.MustCompile(M{"id": inFilter{}}); m.Match(M{"id": "a"}) {
			t.Fatal("struct tagged $in with nil slice field should never match")
		}

		andNil := mongory.MustCompile(M{"$and": nilMaps})
		if !andNil.Match(M{"anything": 1}) || !andNil.Match("scalar") {
			t.Fatal("empty $and from typed nil slice should match everything")
		}
		if m := mongory.MustCompile(M{"profile.$and": nilMaps}); !m.Match(M{"profile": M{}}) {
			t.Fatal("dot-path $and from typed nil slice should match")
		}

		orNil := mongory.MustCompile(M{"$or": nilMaps})
		if orNil.Match(M{"anything": 1}) || orNil.Match(M{}) {
			t.Fatal("empty $or from typed nil slice should never match")
		}
		if m := mongory.MustCompile(M{"profile.$or": nilMaps}); m.Match(M{"profile": M{}}) {
			t.Fatal("dot-path $or from typed nil slice should never match")
		}
	})

	t.Run("untyped nil and non-array nil containers still fail validation", func(t *testing.T) {
		invalid := []any{
			M{"id": M{"$in": nil}},
			M{"id": M{"$nin": nil}},
			M{"$and": nil},
			M{"$or": nil},
			M{"id": M{"$in": nilSlicePtr}},
			M{"id": M{"$in": nilBsonD}},
			M{"$and": nilBsonD},
		}
		for _, cond := range invalid {
			if _, err := mongory.Compile(cond); !errors.Is(err, mongory.ErrInvalidCondition) {
				t.Errorf("Compile(%#v): err = %v, want ErrInvalidCondition", cond, err)
			}
		}
	})

	t.Run("typed nil slice outside list operators preserves nil semantics", func(t *testing.T) {
		mNil := mongory.MustCompile(M{"tags": nilStrings})
		if !mNil.Match(person{Tags: nil}) || !mNil.Match(M{"tags": nil}) || !mNil.Match(M{}) {
			t.Fatal("typed nil slice field condition should match nil/missing field")
		}
		if mNil.Match(person{Tags: []string{}}) {
			t.Fatal("typed nil slice field condition should not match non-nil empty slice")
		}

		zeroStruct := person{Name: "Alice"} // Tags is []string(nil)
		if ok, err := mongory.Match(zeroStruct, zeroStruct); err != nil || !ok {
			t.Fatalf("Match(zeroStruct, zeroStruct) = (%v, %v), want (true, nil)", ok, err)
		}

		mEmpty := mongory.MustCompile(M{"tags": []string{}})
		if !mEmpty.Match(person{Tags: []string{}}) || mEmpty.Match(person{Tags: nil}) {
			t.Fatal("non-nil empty typed slice should match empty slice only")
		}
	})
}

type cycleNode struct {
	Name string
	Next *cycleNode
}

type (
	inlineCycleA struct {
		A             int
		*inlineCycleB `bson:",inline"`
	}
	inlineCycleB struct {
		B             int
		*inlineCycleA `bson:",inline"`
	}
	inlineMapNode struct {
		ID    int
		Extra map[string]any `bson:",inline"`
	}
	selfPtr *selfPtr
)

func TestCycleSafety(t *testing.T) {
	t.Run("cyclic conditions return ErrInvalidCondition", func(t *testing.T) {
		cycMap := M{}
		cycMap["self"] = cycMap

		type recMap map[string]recMap
		cycTypedMap := recMap{}
		cycTypedMap["self"] = cycTypedMap

		type aliasMap map[string]any
		cycAliasMap := M{}
		cycAliasMap["self"] = aliasMap(cycAliasMap)

		cycSlice := make([]any, 1)
		cycSlice[0] = cycSlice

		type recSlice []recSlice
		cycTypedSlice := make(recSlice, 1)
		cycTypedSlice[0] = cycTypedSlice

		cycBsonD := bsonD{{"self", nil}}
		cycBsonD[0].Value = cycBsonD

		cycStruct := &cycleNode{Name: "a"}
		cycStruct.Next = cycStruct

		cycInlineMap := &inlineMapNode{ID: 1}
		cycInlineMap.Extra = M{"self": cycInlineMap}

		var cycPtr selfPtr
		cycPtr = &cycPtr

		var cycIface any
		cycIface = &cycIface

		cases := []struct {
			name string
			cond any
		}{
			{"map self", cycMap},
			{"nested map cycle", M{"$and": A{M{"a": cycMap}}}},
			{"typed map self", cycTypedMap},
			{"aliased map self", cycAliasMap},
			{"slice in $in", M{"id": M{"$in": cycSlice}}},
			{"slice top-level", cycSlice},
			{"typed slice", M{"tags": cycTypedSlice}},
			{"bsonD self", cycBsonD},
			{"struct pointer self", cycStruct},
			{"struct value pointing to cycle", *cycStruct},
			{"struct field nested", M{"node": cycStruct}},
			{"inline map pointing to struct", cycInlineMap},
			{"self-referential pointer type", M{"p": cycPtr}},
			{"self-referential pointer root", cycPtr},
			{"self-referential interface", M{"x": cycIface}},
			{"self-referential interface root", cycIface},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := mongory.Compile(tc.cond); !errors.Is(err, mongory.ErrInvalidCondition) {
					t.Errorf("Compile = %v, want ErrInvalidCondition", err)
				}
			})
		}
	})

	t.Run("mutually recursive inline struct types do not overflow", func(t *testing.T) {
		a := &inlineCycleA{A: 1}
		b := &inlineCycleB{B: 2, inlineCycleA: a}
		a.inlineCycleB = b

		m, err := mongory.Compile(a)
		if err != nil {
			t.Fatalf("Compile(inlineCycleA): %v", err)
		}
		if !m.Match(a) || m.Match(&inlineCycleA{A: 9, inlineCycleB: b}) {
			t.Fatal("Match on mutually recursive inline struct returned unexpected result")
		}
	})

	t.Run("shared DAG and overlapping subslices are allowed", func(t *testing.T) {
		shared := M{"status": "active"}
		addr := &Address{City: "Taipei"}
		buf := make([]any, 2)
		buf[0] = "ok"
		buf[1] = buf[:1]

		m, err := mongory.Compile(M{
			"$and": A{shared, shared},
			"home": addr,
			"work": addr,
			"tags": buf,
		})
		if err != nil {
			t.Fatalf("Compile shared DAG: %v", err)
		}
		rec := M{
			"status": "active",
			"home":   M{"city": "Taipei"},
			"work":   M{"city": "Taipei"},
			"tags":   A{"ok", A{"ok"}},
		}
		if !m.Match(rec) {
			t.Fatal("shared DAG matcher failed to match")
		}
	})

	t.Run("Match and Trace stay safe on cyclic records", func(t *testing.T) {
		n := &cycleNode{Name: "a"}
		n.Next = n

		m := mongory.MustCompile(M{"Next.Next.Name": "a"})
		if !m.Match(n) {
			t.Fatal("Match on cyclic struct record should succeed for finite path")
		}
		if mongory.MustCompile(M{"$eq": M{"Name": "a"}}).Match(n) {
			t.Fatal("whole-object $eq should return false when extra cyclic fields exist")
		}

		var cycPtr selfPtr
		cycPtr = &cycPtr
		var cycIface any
		cycIface = &cycIface
		eqM := mongory.MustCompile(M{"v": 1})
		if eqM.Match(cycPtr) || eqM.Match(cycIface) || eqM.Match(M{"v": cycIface}) {
			t.Fatal("cyclic pointer/interface record should safely evaluate to false")
		}

		cycMap := M{"name": "Ann"}
		cycMap["self"] = cycMap
		cycSlice := make([]any, 1)
		cycSlice[0] = cycSlice
		cycIntMap := map[int]any{}
		cycIntMap[1] = cycIntMap

		ok, trace := mongory.MustCompile(M{"name": "Ann", "tags": 1}).Trace(M{"name": "Ann", "self": cycMap, "tags": cycSlice, "meta": cycIntMap})
		if ok || !strings.Contains(trace, "<cycle>") {
			t.Fatalf("expected trace with <cycle>, got ok=%v:\n%s", ok, trace)
		}
		if _, trace2 := mongory.MustCompile(M{"meta": 1}).Trace(M{"meta": cycIntMap}); !strings.Contains(trace2, "<cycle>") {
			t.Fatalf("expected trace with <cycle> for non-string cyclic map:\n%s", trace2)
		}
	})
}

func makeNestedCondition(depth int) M {
	cur := M{"leaf": 1}
	for range depth - 1 {
		cur = M{"next": cur}
	}
	return cur
}

func TestCompileLimits(t *testing.T) {
	t.Run("default max depth enforces 64 and rejects 65", func(t *testing.T) {
		okCond := makeNestedCondition(mongory.DefaultMaxDepth)
		if _, err := mongory.Compile(okCond); err != nil {
			t.Fatalf("depth %d should succeed, got %v", mongory.DefaultMaxDepth, err)
		}

		tooDeep := makeNestedCondition(mongory.DefaultMaxDepth + 1)
		if _, err := mongory.Compile(tooDeep); !errors.Is(err, mongory.ErrLimitExceeded) || !errors.Is(err, mongory.ErrInvalidCondition) {
			t.Fatalf("depth %d: err = %v, want ErrLimitExceeded and ErrInvalidCondition", mongory.DefaultMaxDepth+1, err)
		}

		dotKeys := make([]string, mongory.DefaultMaxDepth+1)
		for i := range dotKeys {
			dotKeys[i] = "a"
		}
		if _, err := mongory.Compile(M{strings.Join(dotKeys, "."): 1}); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("deep dotted key should fail with ErrLimitExceeded, got %v", err)
		}

		var deepSlice any = 1
		for range mongory.DefaultMaxDepth + 2 {
			deepSlice = A{deepSlice}
		}
		if _, err := mongory.Compile(M{"x": M{"$in": deepSlice}}); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("deep nested slice should fail with ErrLimitExceeded, got %v", err)
		}
	})

	t.Run("WithMaxDepth overrides and non-positive disables limit", func(t *testing.T) {
		if _, err := mongory.Compile(M{}, mongory.WithMaxDepth(1), mongory.WithMaxNodes(1)); err != nil {
			t.Fatalf("empty condition failed minimal limits: %v", err)
		}

		depth3 := makeNestedCondition(3)
		m, err := mongory.Compile(depth3, mongory.WithMaxDepth(3))
		if err != nil {
			t.Fatalf("depth 3 with MaxDepth(3) failed: %v", err)
		}
		if !m.Match(makeNestedCondition(3)) {
			t.Fatal("compiled depth-3 matcher did not match")
		}

		if _, err := mongory.Compile(M{"a.b.c": 1}, mongory.WithMaxDepth(3)); err != nil {
			t.Fatalf("dot path depth 3 with MaxDepth(3) failed: %v", err)
		}
		if _, err := mongory.Compile(M{"a.b.c.d": 1}, mongory.WithMaxDepth(3)); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("dot path depth 4 with MaxDepth(3) should fail, got %v", err)
		}
		if _, err := mongory.Compile(M{"$or": A{M{"$and": A{M{"a": 1}}}}}, mongory.WithMaxDepth(3)); err != nil {
			t.Fatalf("depth 3 composite with MaxDepth(3) failed: %v", err)
		}
		if _, err := mongory.Compile(M{"a": M{"b": M{"$not": M{"$gt": 1}}}}, mongory.WithMaxDepth(3)); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("depth 4 operator with MaxDepth(3) should fail, got %v", err)
		}
		if _, err := mongory.Compile(M{"a": M{"$eq": makeNestedCondition(5)}}, mongory.WithMaxDepth(4)); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("deep $eq value should respect MaxDepth, got %v", err)
		}

		deep80 := makeNestedCondition(80)
		m80, err := mongory.Compile(deep80, mongory.WithMaxDepth(0))
		if err != nil {
			t.Fatalf("WithMaxDepth(0) should allow deep query, got %v", err)
		}
		if !m80.Match(deep80) {
			t.Fatal("deep 80 matcher did not match")
		}
		if ok, err := mongory.Match(M{"a.b.c": 1}, M{"a": M{"b": M{"c": 1}}}, mongory.WithMaxDepth(2)); ok || !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("one-shot Match should enforce WithMaxDepth, got (%v, %v)", ok, err)
		}
	})

	t.Run("MaxNodes rejects wide trees and exempts large $in lists", func(t *testing.T) {
		m5, err := mongory.Compile(M{"a": 1, "b": 2}, mongory.WithMaxNodes(5))
		if err != nil || !m5.Match(M{"a": 1, "b": 2}) {
			t.Fatalf("5-node condition with MaxNodes(5) failed: %v", err)
		}
		if _, err := mongory.Compile(M{"a": 1, "b": 2, "c": 3}, mongory.WithMaxNodes(5)); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("7-node condition with MaxNodes(5) should fail, got %v", err)
		}

		large := make(A, 5000)
		for i := range large {
			large[i] = i
		}
		if m, err := mongory.Compile(M{"id": M{"$in": large}}, mongory.WithMaxNodes(3)); err != nil || !m.Match(M{"id": 4999}) {
			t.Fatalf("large $in should count as 1 matcher node: %v", err)
		}

		wide := make(A, 2500)
		for i := range wide {
			wide[i] = M{"k" + strconv.Itoa(i): i}
		}
		if _, err := mongory.Compile(M{"$or": wide}); !errors.Is(err, mongory.ErrLimitExceeded) {
			t.Fatalf("wide $or exceeding DefaultMaxNodes should fail, got %v", err)
		}
		if _, err := mongory.Compile(M{"$or": wide}, mongory.WithMaxNodes(-1)); err != nil {
			t.Fatalf("WithMaxNodes(-1) should disable node limit, got %v", err)
		}
	})
}

func TestLargeInLookup(t *testing.T) {
	now := time.Now()
	laterUTC := time.Date(2026, 6, 15, 12, 0, 0, 123_000_000, time.UTC)
	oidMatch := objectID{1, 2, 3, 4}
	oidOther := objectID{9, 9, 9, 9}
	reA := regexp.MustCompile("^alpha")

	// Build a > 16 item mixed list to trigger compile-time hash lookup alongside
	// fallback containers.
	baseItems := A{
		nil,
		true,
		"hit",
		42,
		int64(9007199254740993),   // 2^53 + 1
		float64(9007199254740992), // 2^53 (exact integral float)
		float64(30.0),
		float64(30.5),
		math.Copysign(0, -1), // -0.0
		math.NaN(),
		math.Inf(1),
		uint64(math.MaxUint64),
		now,
		laterUTC,
		oidMatch,
		M{"k": 1, "m": "x"},
		A{1, "two"},
		reA,
	}
	for i := range 20 {
		baseItems = append(baseItems, "pad_"+strconv.Itoa(i))
	}

	inM := mongory.MustCompile(M{"v": M{"$in": baseItems}})
	ninM := mongory.MustCompile(M{"v": M{"$nin": baseItems}})

	type recHolder struct {
		V any `bson:"v"`
	}

	tests := []struct {
		name string
		rec  any
		want bool
	}{
		{"nil value matches nil in $in", M{"v": nil}, true},
		{"missing key never matches nil in $in", M{}, false},
		{"nil pointer matches nil in $in", recHolder{V: (*int)(nil)}, true},
		{"typed nil slice matches nil in $in", recHolder{V: ([]string)(nil)}, true},
		{"bool true matches", M{"v": true}, true},
		{"bool false mismatches", M{"v": false}, false},
		{"string matches", M{"v": "hit"}, true},
		{"pad string matches", M{"v": "pad_19"}, true},
		{"string misses", M{"v": "miss"}, false},
		{"int matches int", M{"v": 42}, true},
		{"int matches integral float", M{"v": 42.0}, true},
		{"int8 matches via reflect", recHolder{V: int8(42)}, true},
		{"json.Number int matches", M{"v": json.Number("42")}, true},
		{"json.Number float matches int", M{"v": json.Number("42.0")}, true},
		{"integral float matches int record", M{"v": 30}, true},
		{"integral float matches float record", M{"v": 30.0}, true},
		{"fractional float matches", M{"v": 30.5}, true},
		{"2^53 boundary rejects adjacent float for int item", M{"v": float64(9007199254740992)}, true},
		{"2^53 + 1 exact int matches int", M{"v": int64(9007199254740993)}, true},
		{"2^53+1 does not match 2^53 float", mongory.M{"v": float64(9007199254740994)}, false},
		{"-0.0 matches int 0", M{"v": 0}, true},
		{"-0.0 matches +0.0", M{"v": 0.0}, true},
		{"NaN never matches NaN", M{"v": math.NaN()}, false},
		{"+Inf matches +Inf", M{"v": math.Inf(1)}, true},
		{"MaxUint64 matches float", M{"v": uint64(math.MaxUint64)}, true},
		{"MaxInt64 rejects MaxUint64 float", M{"v": int64(math.MaxInt64)}, false},
		{"time.Now monotonic stripped matches UTC copy", M{"v": now.Round(0).UTC()}, true},
		{"time matches fixed timezone copy", M{"v": laterUTC.In(time.FixedZone("x", 3600))}, true},
		{"DateTime timeValuer matches time.Time", M{"v": dateTime(laterUTC.UnixMilli())}, true},
		{"ObjectID matches via hash", M{"v": oidMatch}, true},
		{"ObjectID misses", M{"v": oidOther}, false},
		{"different named [4]byte misses", M{"v": [4]byte{1, 2, 3, 4}}, false},
		{"map fallback key order independent", M{"v": M{"m": "x", "k": 1}}, true},
		{"nested slice element in array fallback matches", M{"v": A{A{1, "two"}}}, true},
		{"typed nested slice element in array fallback matches", M{"v": []any{[]int{1}, A{1, "two"}}}, true},
		{"flat array checks element intersection only", M{"v": A{1, "two"}}, false},
		{"regexp pointer fallback matches", M{"v": reA}, true},
		{"array intersection with large $in", M{"v": []string{"no", "pad_12"}}, true},
		{"array disjoint with large $in", M{"v": []string{"no", "nope"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inM.Match(tt.rec); got != tt.want {
				t.Errorf("$in Match(%#v) = %v, want %v", tt.rec, got, tt.want)
			}
			if got := ninM.Match(tt.rec); got != !tt.want {
				t.Errorf("$nin Match(%#v) = %v, want %v", tt.rec, got, !tt.want)
			}
		})
	}

	t.Run("large $in without false 2^53 rounding match", func(t *testing.T) {
		items := make(A, 20)
		for i := range items {
			items[i] = i
		}
		items[0] = float64(9007199254740992) // 2^53
		m := mongory.MustCompile(M{"n": M{"$in": items}})
		if m.Match(M{"n": int64(9007199254740993)}) {
			t.Fatal("2^53 float item must not match 2^53+1 int64 record")
		}
		if !m.Match(M{"n": int64(9007199254740992)}) {
			t.Fatal("2^53 float item should match exact 2^53 int64 record")
		}
	})

	t.Run("indexed $in orders before $regex", func(t *testing.T) {
		items := make(A, 100)
		for i := range items {
			items[i] = "id-" + strconv.Itoa(i)
		}
		m := mongory.MustCompile(M{
			"name": M{"$regex": "^user"},
			"code": M{"$in": items},
		})
		explain := m.Explain()
		inIdx := strings.Index(explain, `Field("code")`)
		reIdx := strings.Index(explain, `Field("name")`)
		if inIdx < 0 || reIdx < 0 || inIdx > reIdx {
			t.Fatalf("expected O(1) $in Field(\"code\") before $regex Field(\"name\"):\n%s", explain)
		}
	})
}
