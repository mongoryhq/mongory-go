package mongory_test

import (
	"fmt"

	"github.com/mongoryhq/mongory-go"
)

type User struct {
	Name   string   `bson:"name"`
	Age    int      `bson:"age"`
	Tags   []string `bson:"tags"`
	Active bool     // no tag: key is "Active" (exact Go field name)
}

var adults = mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})

func ExampleFilter() {
	users := []User{
		{Name: "Ann", Age: 30, Tags: []string{"go"}, Active: true},
		{Name: "Bob", Age: 15, Tags: []string{"ruby"}, Active: true},
		{Name: "Cid", Age: 42, Tags: []string{"go", "c"}},
	}

	m, err := mongory.Compile(mongory.M{
		"Active": true,
		"$or": mongory.A{
			mongory.M{"tags": "go"},
			mongory.M{"age": mongory.M{"$lt": 18}},
		},
	})
	if err != nil {
		panic(err)
	}
	for _, u := range mongory.Filter(users, m) {
		fmt.Println(u.Name)
	}

	fmt.Println(mongory.Count(users, adults))
	// Output:
	// Ann
	// Bob
	// 2
}

func ExampleMatcher_Match() {
	m := mongory.MustCompile(mongory.M{"address.city": "Taipei", "tags": mongory.M{"$size": 2}})

	fmt.Println(m.Match(mongory.M{
		"address": mongory.M{"city": "Taipei"},
		"tags":    []string{"go", "ruby"},
	}))
	fmt.Println(m.Match(mongory.M{"address": mongory.M{"city": "Tokyo"}}))
	// Output:
	// true
	// false
}

func ExampleMatch() {
	ok, err := mongory.Match(mongory.M{"age": mongory.M{"$gte": 18}}, mongory.M{"age": 21})
	fmt.Println(ok, err)
	// Output:
	// true <nil>
}

func ExampleCompile() {
	_, err := mongory.Compile(mongory.M{"age": mongory.M{"$in": 18}})
	fmt.Println(err)
	// Output:
	// mongory: invalid condition: $in needs an array, got int64
}

func ExampleMatcher_Explain() {
	m := mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}, "status": "active"})
	fmt.Print(m.Explain())
	// Output:
	// And
	// ├── Field("status")
	// │   └── Eq("active")
	// └── Field("age")
	//     └── Gte(18)
}

func ExampleMatcher_Trace() {
	m := mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}, "status": "active"})
	ok, trace := m.Trace(mongory.M{"age": 15, "status": "active"})
	fmt.Println(ok)
	fmt.Print(trace)
	// Output:
	// false
	// [FAIL] And | value={"age": 15, "status": "active"}
	// ├── [PASS] Field("status") | value="active"
	// │   └── [PASS] Eq("active") | value="active"
	// └── [FAIL] Field("age") | value=15
	//     └── [FAIL] Gte(18) | value=15
}

func ExampleMatcher_TraceTree() {
	m := mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}, "status": "active"})
	ok, tree := m.TraceTree(mongory.M{"age": 15, "status": "active"})
	fmt.Println(ok, tree.Kind, tree.Status, tree.Children[1].Field, tree.Children[1].Status)
	// Output:
	// false and fail age fail
}

func ExampleWithOptions() {
	type Money struct {
		Currency string
		Cents    int64
	}
	preset := mongory.WithOptions(
		mongory.WithConverter(func(m *Money) any { return m.Cents }),
		mongory.WithOperator("$currency", mongory.Operator{
			Compile: func(arg any) (mongory.Predicate, error) {
				want := arg.(string)
				return func(v any, exists bool) bool {
					m, ok := v.(*Money)
					return exists && ok && m != nil && m.Currency == want
				}, nil
			},
		}),
	)

	m := mongory.MustCompile(mongory.M{
		"price": mongory.M{"$gte": 1000, "$currency": "TWD"},
	}, preset)

	fmt.Println(m.Match(mongory.M{"price": &Money{Currency: "TWD", Cents: 1500}}))
	fmt.Println(m.Match(mongory.M{"price": &Money{Currency: "USD", Cents: 1500}}))
	// Output:
	// true
	// false
}

func ExampleAnd() {
	base := mongory.M{"status": mongory.M{"$ne": "removed"}}
	extra := mongory.M{"status": mongory.M{"$in": mongory.A{"active", "pending"}}}
	combined := mongory.And(base, nil, extra)
	m := mongory.MustCompile(combined)
	fmt.Println(m.Match(mongory.M{"status": "active"}))
	fmt.Println(m.Match(mongory.M{"status": "removed"}))
	// Output:
	// true
	// false
}
