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

var adults = mongory.MustNew(mongory.M{"age": mongory.M{"$gte": 18}})

func ExampleFilter() {
	users := []User{
		{Name: "Ann", Age: 30, Tags: []string{"go"}, Active: true},
		{Name: "Bob", Age: 15, Tags: []string{"ruby"}, Active: true},
		{Name: "Cid", Age: 42, Tags: []string{"go", "c"}},
	}

	q, err := mongory.New(mongory.M{
		"active": true,
		"$or": mongory.A{
			mongory.M{"tags": "go"},
			mongory.M{"age": mongory.M{"$lt": 18}},
		},
	})
	if err != nil {
		panic(err)
	}
	for _, u := range mongory.Filter(users, q) {
		fmt.Println(u.Name)
	}

	fmt.Println(len(mongory.Filter(users, adults)))
	// Output:
	// Ann
	// Bob
	// 2
}

func ExampleQuery_Match() {
	q := mongory.MustNew(mongory.M{"address.city": "Taipei", "tags": mongory.M{"$size": 2}})

	fmt.Println(q.Match(mongory.M{
		"address": mongory.M{"city": "Taipei"},
		"tags":    []string{"go", "ruby"},
	}))
	fmt.Println(q.Match(mongory.M{"address": mongory.M{"city": "Tokyo"}}))
	// Output:
	// true
	// false
}

func ExampleNew() {
	_, err := mongory.New(mongory.M{"age": mongory.M{"$in": 18}})
	fmt.Println(err)
	// Output:
	// mongory: invalid condition: $in needs an array, got int64
}
