package mongory_test

import (
	"encoding/json"
	"errors"
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
	Base     `bson:",inline"`
	Meta                    // embedded without inline: a "meta" subdocument, as in the driver
	Name     string         `mongory:"full_name" bson:"name"`
	Age      uint8          `bson:"age,omitempty"`
	Tags     []string       `bson:"tags"`
	Address  *Address       `bson:"address"`
	Secret   string         `bson:"-"`
	Nickname string         `json:"nick"` // json tag ignored: key "nickname"
	Extra    map[string]any `bson:",inline"`
	hidden   string
}

type role string

type wrapper struct {
	*Base `bson:",inline"`
	Role  role
}

// Driver look-alikes: matched by shape, so no mongo-driver dependency.
type (
	bsonE struct {
		Key   string
		Value any
	}
	bsonD    []bsonE
	objectID [12]byte
	dateTime int64
)

func (d dateTime) Time() time.Time { return time.UnixMilli(int64(d)).UTC() }

func TestQueryMatch(t *testing.T) {
	alice := person{
		Base:     Base{ID: 7},
		Meta:     Meta{Source: "web"},
		Name:     "Alice",
		Age:      30,
		Tags:     []string{"go", "ruby"},
		Address:  &Address{City: "Taipei"},
		Secret:   "s",
		Nickname: "al",
		Extra:    M{"plan": "pro", "full_name": "shadowed"},
		hidden:   "h",
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
		{"struct mongory tag wins", M{"full_name": "Alice"}, alice, true},
		{"struct bson tag", M{"age": 30, "address.city": "Taipei"}, alice, true},
		{"struct pointer", M{"age": 30}, &alice, true},
		{"struct typed slice", M{"tags": "ruby"}, alice, true},
		{"struct inline embedded", M{"_id": 7}, alice, true},
		{"struct embedded is subdocument", M{"meta.source": "web"}, alice, true},
		{"struct embedded not flattened", M{"source": nil}, alice, true},
		{"struct untagged key is lowercased", M{"nickname": "al"}, alice, true},
		{"struct json tag ignored", M{"nick": nil}, alice, true},
		{"struct dash tag skipped", M{"secret": nil}, alice, true},
		{"struct unexported skipped", M{"hidden": nil}, alice, true},
		{"struct inline map", M{"plan": "pro"}, alice, true},
		{"struct inline map never overrides fields", M{"full_name": "Alice"}, alice, true},
		{"struct nil inline pointer", M{"_id": nil, "role": "admin"}, wrapper{Role: "admin"}, true},
		{"struct as condition", Meta{Source: "web"}, M{"source": "web"}, true},
		{"nil pointer record", M{"a": 1}, (*person)(nil), false},
		{"bson.D condition", bsonD{{"age", 30}, {"name", "Al"}}, M{"age": 30, "name": "Al"}, true},
		{"bson.D condition mismatch", bsonD{{"age", 30}}, M{"age": 31}, false},
		{"nested bson.D condition", M{"xs": bsonD{{"$elemMatch", bsonD{{"a", 1}}}}}, M{"xs": A{M{"a": 1}}}, true},
		{"bson.D record", M{"a.b": 1}, bsonD{{"a", bsonD{{"b", 1}}}}, true},
		{"ObjectID equality", M{"_id": oid}, M{"_id": oid}, true},
		{"ObjectID inequality", M{"_id": oid}, M{"_id": objectID{9}}, false},
		{"ObjectID is not an array", M{"_id.0": 1}, M{"_id": oid}, false},
		{"DateTime equals time.Time", M{"at": createdAt}, M{"at": dateTime(createdAt.UnixMilli())}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := mongory.New(tt.condition)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := q.Match(tt.record); got != tt.want {
				t.Errorf("Match(%#v) = %v, want %v", tt.record, got, tt.want)
			}
		})
	}
}

func TestNewInvalidCondition(t *testing.T) {
	_, err := mongory.New(M{"xs": M{"$elemMatch": 1}})
	if !errors.Is(err, mongory.ErrInvalidCondition) {
		t.Fatalf("err = %v, want mongory.ErrInvalidCondition", err)
	}
}

func TestQueryDoesNotMutateCondition(t *testing.T) {
	cond := M{"a.b": M{"c.d": 1}}
	if _, err := mongory.New(cond); err != nil {
		t.Fatal(err)
	}
	if _, ok := cond["a.b"].(M)["c.d"]; !ok {
		t.Fatalf("condition mutated: %#v", cond)
	}
}
