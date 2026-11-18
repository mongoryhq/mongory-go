package mongory_test

import (
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
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

func makeLargeInTargets(n int) A {
	items := make(A, n)
	items[0] = "usera"
	items[1] = "userb"
	for i := 2; i < n; i++ {
		items[i] = "other-" + strconv.Itoa(i)
	}
	return items
}

var (
	benchRegex      = regexp.MustCompile(`^user[a-f]$`)
	benchLargeInSet = func() map[string]struct{} {
		items := makeLargeInTargets(500)
		s := make(map[string]struct{}, len(items))
		for _, v := range items {
			s[v.(string)] = struct{}{}
		}
		return s
	}()
)

var benchCases = []struct {
	name        string
	condition   M
	plainMap    func(M) bool
	plainStruct func(*benchUser) bool
}{
	{
		name:      "simple",
		condition: M{"age": M{"$gte": 18}},
		plainMap: func(r M) bool {
			age, ok := r["age"].(int)
			return ok && age >= 18
		},
		plainStruct: func(u *benchUser) bool { return u.Age != nil && *u.Age >= 18 },
	},
	{
		name:      "or",
		condition: M{"$or": A{M{"age": M{"$gte": 18}}, M{"status": "active"}}},
		plainMap: func(r M) bool {
			if age, ok := r["age"].(int); ok && age >= 18 {
				return true
			}
			return r["status"] == "active"
		},
		plainStruct: func(u *benchUser) bool {
			return (u.Age != nil && *u.Age >= 18) || u.Status == "active"
		},
	},
	{
		name:      "nested_and",
		condition: M{"profile.city": "Taipei", "status": "active", "age": M{"$lt": 40}},
		plainMap: func(r M) bool {
			if r["status"] != "active" {
				return false
			}
			age, ok := r["age"].(int)
			if !ok || age >= 40 {
				return false
			}
			p, ok := r["profile"].(M)
			return ok && p["city"] == "Taipei"
		},
		plainStruct: func(u *benchUser) bool {
			return u.Status == "active" && u.Age != nil && *u.Age < 40 && u.Profile.City == "Taipei"
		},
	},
	{
		name:      "in_array_field",
		condition: M{"tags": M{"$in": A{"go", "rust"}}},
		plainMap: func(r M) bool {
			tags, ok := r["tags"].([]any)
			if !ok {
				return false
			}
			for _, t := range tags {
				if t == "go" || t == "rust" {
					return true
				}
			}
			return false
		},
		plainStruct: func(u *benchUser) bool {
			return slices.Contains(u.Tags, "go") || slices.Contains(u.Tags, "rust")
		},
	},
	{
		name:      "large_in",
		condition: M{"name": M{"$in": makeLargeInTargets(500)}},
		plainMap: func(r M) bool {
			s, ok := r["name"].(string)
			if !ok {
				return false
			}
			_, hit := benchLargeInSet[s]
			return hit
		},
		plainStruct: func(u *benchUser) bool {
			_, hit := benchLargeInSet[u.Name]
			return hit
		},
	},
	{
		name:      "regex",
		condition: M{"name": M{"$regex": "^user[a-f]$"}},
		plainMap: func(r M) bool {
			s, ok := r["name"].(string)
			return ok && benchRegex.MatchString(s)
		},
		plainStruct: func(u *benchUser) bool { return benchRegex.MatchString(u.Name) },
	},
}

func BenchmarkMatch(b *testing.B) {
	maps, users := benchData()
	for _, bc := range benchCases {
		m := mongory.MustCompile(bc.condition)
		want := countMatches(m, maps)
		if got := countMatches(m, users); got != want {
			b.Fatalf("%s: structs matched %d, maps matched %d", bc.name, got, want)
		}

		b.Run(bc.name+"/plain_maps", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				n := 0
				for _, r := range maps {
					if bc.plainMap(r) {
						n++
					}
				}
				if n != want {
					b.Fatalf("plain_maps got %d, want %d", n, want)
				}
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/maps", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				countMatches(m, maps)
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/plain_structs", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				n := 0
				for idx := range users {
					if bc.plainStruct(&users[idx]) {
						n++
					}
				}
				if n != want {
					b.Fatalf("plain_structs got %d, want %d", n, want)
				}
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/structs", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				countMatches(m, users)
			}
			reportPerRecord(b)
		})
		b.Run(bc.name+"/structs_count", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if got := mongory.Count(users, m); got != want {
					b.Fatalf("got %d, want %d", got, want)
				}
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

func BenchmarkOptionsFastPath(b *testing.B) {
	_, users := benchData()
	emptyPreset := mongory.WithOptions()
	m := mongory.MustCompile(M{"profile.city": "Taipei", "status": "active", "age": M{"$lt": 40}}, emptyPreset)
	b.Run("empty_preset/structs_count", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mongory.Count(users, m)
		}
		reportPerRecord(b)
	})
	b.Run("structs_filter_limit_10", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mongory.FilterLimit(users, m, 10)
		}
	})
}
