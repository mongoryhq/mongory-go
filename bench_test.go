package mongory_test

import (
	"math/rand/v2"
	"testing"

	"github.com/mongoryhq/mongory-go"
)

// Compares the pure Go engine with the legacy CGO matcher on the same data.
//
//	go test -run '^$' -bench . -benchmem

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

// benchData returns the same records as maps and as structs.
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
	skipCGO   string // why the legacy CGO binding cannot run this case
}{
	{name: "simple", condition: M{"age": M{"$gte": 18}}},
	{name: "or", condition: M{"$or": A{M{"age": M{"$gte": 18}}, M{"status": "active"}}}},
	// Nested object instead of "profile.city": the CGO binding does not expand dot paths.
	{name: "nested_and", condition: M{"profile": M{"city": "Taipei"}, "status": "active", "age": M{"$lt": 40}}},
	{name: "in_array_field", condition: M{"tags": M{"$in": A{"go", "rust"}}}},
	{name: "regex", condition: M{"name": M{"$regex": "^user[a-f]$"}}, skipCGO: "the CGO binding registers no regex adapter"},
}

func BenchmarkMatch(b *testing.B) {
	maps, users := benchData()
	for _, bc := range benchCases {
		pure := mongory.MustNew(bc.condition)
		want := countPure(pure, maps)
		if got := countPure(pure, users); got != want {
			b.Fatalf("%s: structs matched %d, maps matched %d", bc.name, got, want)
		}

		b.Run(bc.name+"/pure_maps", func(b *testing.B) {
			for b.Loop() {
				countPure(pure, maps)
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/pure_structs", func(b *testing.B) {
			for b.Loop() {
				countPure(pure, users)
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/cgo_maps", func(b *testing.B) {
			if bc.skipCGO != "" {
				b.Skip(bc.skipCGO)
			}
			legacy, err := mongory.NewCMatcher(bc.condition, nil)
			if err != nil {
				b.Fatalf("NewCMatcher: %v", err)
			}
			if got := countLegacy(b, legacy, maps); got != want {
				b.Fatalf("CGO matched %d, pure Go matched %d", got, want)
			}
			for b.Loop() {
				countLegacy(b, legacy, maps)
			}
			reportPerRecord(b)
		})
	}
}

func countPure[T any](q *mongory.Query, records []T) int {
	n := 0
	for _, r := range records {
		if q.Match(r) {
			n++
		}
	}
	return n
}

func countLegacy(b *testing.B, m mongory.CMatcher, records []M) int {
	n := 0
	for _, r := range records {
		ok, err := m.Match(r)
		if err != nil {
			b.Fatal(err)
		}
		if ok {
			n++
		}
	}
	return n
}

func reportPerRecord(b *testing.B) {
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchSize, "ns/record")
}
