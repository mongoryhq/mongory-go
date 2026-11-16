package mongory_test

import (
	"math/rand/v2"
	"testing"

	"github.com/mongoryhq/mongory-go"
)

// Run with: go test -run '^$' -bench . -benchmem

const benchSize = 10_000

type benchProfile struct {
	City string `bson:"city"`
}

type benchUser struct {
	Name    string       `bson:"name"`
	Age     *int         `bson:"age"`
	Status  string       `bson:"status"`
	Tags    []string     `bson:"tags"`
	Profile benchProfile `bson:"profile"`
}

var (
	benchCities = []string{"Taipei", "Tokyo", "Seoul", "Paris", "Lima"}
	benchTags   = []string{"go", "ruby", "c", "js", "rust", "python"}
)

func benchData() ([]M, []benchUser) {
	r := rand.New(rand.NewPCG(1, 2))
	maps := make([]M, benchSize)
	users := make([]benchUser, benchSize)
	for i := range maps {
		u := benchUser{
			Name:    "user" + string(rune('a'+r.IntN(26))),
			Status:  []string{"active", "inactive"}[r.IntN(2)],
			Profile: benchProfile{City: benchCities[r.IntN(len(benchCities))]},
		}
		if r.IntN(2) == 0 {
			age := r.IntN(100) + 1
			u.Age = &age
		}
		for range r.IntN(4) {
			u.Tags = append(u.Tags, benchTags[r.IntN(len(benchTags))])
		}
		users[i] = u

		m := M{"name": u.Name, "status": u.Status, "profile": M{"city": u.Profile.City}, "age": nil}
		if u.Age != nil {
			m["age"] = *u.Age
		}
		tags := make([]any, len(u.Tags))
		for j, t := range u.Tags {
			tags[j] = t
		}
		m["tags"] = tags
		maps[i] = m
	}
	return maps, users
}

var benchCases = []struct {
	name      string
	condition M
}{
	{name: "simple", condition: M{"age": M{"$gte": 18}}},
	{name: "or", condition: M{"$or": A{M{"age": M{"$gte": 18}}, M{"status": "active"}}}},
	{name: "nested_and", condition: M{"profile.city": "Taipei", "status": "active", "age": M{"$lt": 40}}},
	{name: "in_array_field", condition: M{"tags": M{"$in": A{"go", "rust"}}}},
	{name: "regex", condition: M{"name": M{"$regex": "^user[a-f]$"}}},
}

func BenchmarkMatch(b *testing.B) {
	maps, users := benchData()
	for _, bc := range benchCases {
		m := mongory.MustCompile(bc.condition)
		want := countMatches(m, maps)
		if got := countMatches(m, users); got != want {
			b.Fatalf("%s: structs matched %d, maps matched %d", bc.name, got, want)
		}

		b.Run(bc.name+"/maps", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				countMatches(m, maps)
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/structs", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				countMatches(m, users)
			}
			reportPerRecord(b)
		})
	}
}

func countMatches[T any](m *mongory.Matcher, records []T) int {
	n := 0
	for _, r := range records {
		if m.Match(r) {
			n++
		}
	}
	return n
}

func reportPerRecord(b *testing.B) {
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchSize, "ns/record")
}
