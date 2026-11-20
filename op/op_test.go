package op_test

import (
	"testing"

	"github.com/mongoryhq/mongory-go"
	"github.com/mongoryhq/mongory-go/op"
)

func TestOperatorConstants(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Eq", op.Eq, "$eq"},
		{"Ne", op.Ne, "$ne"},
		{"Gt", op.Gt, "$gt"},
		{"Gte", op.Gte, "$gte"},
		{"Lt", op.Lt, "$lt"},
		{"Lte", op.Lte, "$lte"},
		{"In", op.In, "$in"},
		{"Nin", op.Nin, "$nin"},
		{"Exists", op.Exists, "$exists"},
		{"Present", op.Present, "$present"},
		{"And", op.And, "$and"},
		{"Or", op.Or, "$or"},
		{"Not", op.Not, "$not"},
		{"ElemMatch", op.ElemMatch, "$elemMatch"},
		{"Every", op.Every, "$every"},
		{"Size", op.Size, "$size"},
		{"Regex", op.Regex, "$regex"},
		{"Options", op.Options, "$options"},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestOperatorConstantsWithCompile(t *testing.T) {
	t.Parallel()

	cond := mongory.M{
		op.And: mongory.A{
			mongory.M{"age": mongory.M{op.Gte: 18, op.Lt: 65, op.Gt: 17, op.Lte: 64, op.Ne: 20}},
			mongory.M{"role": mongory.M{op.In: mongory.A{"admin", "editor"}, op.Nin: mongory.A{"guest"}}},
			mongory.M{"nickname": mongory.M{op.Exists: false}},
			mongory.M{"name": mongory.M{op.Present: true, op.Regex: "^alice$", op.Options: "i"}},
			mongory.M{"status": mongory.M{op.Not: mongory.M{op.Eq: "banned"}}},
			mongory.M{"scores": mongory.M{op.Size: 2, op.Every: mongory.M{op.Gte: 80}, op.ElemMatch: mongory.M{op.Gt: 90}}},
			mongory.M{op.Or: mongory.A{
				mongory.M{"active": true},
				mongory.M{"vip": true},
			}},
		},
	}

	m, err := mongory.Compile(cond)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	good := mongory.M{
		"age":    30,
		"role":   "admin",
		"name":   "Alice",
		"status": "active",
		"scores": mongory.A{85, 95},
		"active": true,
	}
	if !m.Match(good) {
		t.Fatalf("expected record to match")
	}

	bad := mongory.M{
		"age":    30,
		"role":   "admin",
		"name":   "Alice",
		"status": "banned",
		"scores": mongory.A{85, 95},
		"active": true,
	}
	if m.Match(bad) {
		t.Fatalf("expected banned record not to match")
	}
}
