package mongory_test

import (
	"fmt"

	"github.com/mongoryhq/mongory-go"
)

type User struct {
	Name   string   `bson:"name"`
	Age    int      `bson:"age"`
	Tags   []string `bson:"tags"`
	Active bool     // no tag: key is "active", as in the mongo-go-driver
}

var adults = mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})

func ExampleFilter() {
	users := []User{
		{Name: "Ann", Age: 30, Tags: []string{"go"}, Active: true},
		{Name: "Bob", Age: 15, Tags: []string{"ruby"}, Active: true},
		{Name: "Cid", Age: 42, Tags: []string{"go", "c"}},
	}

	m, err := mongory.Compile(mongory.M{
		"active": true,
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
