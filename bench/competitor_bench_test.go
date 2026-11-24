package bench_test

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/256dpi/lungo/bsonkit"
	"github.com/256dpi/lungo/mongokit"
	"github.com/SierraSoftworks/connor"
	"github.com/mongoryhq/mongory-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const benchSize = 10_000

var (
	benchCities = []string{"Taipei", "Tokyo", "Seoul", "Paris", "Lima"}
	benchTags   = []string{"go", "ruby", "c", "js", "rust", "python"}
)

func benchData() ([]mongory.M, []bsonkit.Doc) {
	r := rand.New(rand.NewPCG(1, 2))
	maps := make([]mongory.M, benchSize)
	docs := make([]bsonkit.Doc, benchSize)
	for i := range maps {
		name := "user" + string(rune('a'+r.IntN(26)))
		status := []string{"active", "inactive"}[r.IntN(2)]
		city := benchCities[r.IntN(len(benchCities))]
		var age any
		if r.IntN(2) == 0 {
			age = r.IntN(100) + 1
		}
		tagCount := r.IntN(4)
		tags := make([]any, tagCount)
		for j := range tagCount {
			tags[j] = benchTags[r.IntN(len(benchTags))]
		}
		m := mongory.M{
			"name":    name,
			"status":  status,
			"profile": mongory.M{"city": city},
			"age":     age,
			"tags":    tags,
		}
		maps[i] = m
		doc, err := bsonkit.Convert(map[string]any(m))
		if err != nil {
			panic(err)
		}
		docs[i] = doc
	}
	return maps, docs
}

func makeLargeInTargets(n int) mongory.A {
	items := make(mongory.A, n)
	items[0] = "usera"
	items[1] = "userb"
	for i := 2; i < n; i++ {
		items[i] = "other-" + strconv.Itoa(i)
	}
	return items
}

func toPlainMap(m mongory.M) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case mongory.M:
			out[k] = toPlainMap(x)
		case mongory.A:
			arr := make([]any, len(x))
			for i, elem := range x {
				if sub, ok := elem.(mongory.M); ok {
					arr[i] = toPlainMap(sub)
				} else {
					arr[i] = elem
				}
			}
			out[k] = arr
		default:
			out[k] = v
		}
	}
	return out
}

type peerCase struct {
	name       string
	condition  mongory.M
	lungoCond  map[string]any // optional override when lungo requires BSON primitive types
	connorCond map[string]any // optional override for connor's operator dialect
	skipConnor bool
	skipLungo  bool
}

var peerCases = []peerCase{
	{
		name:       "simple",
		condition:  mongory.M{"age": mongory.M{"$gte": 18}},
		connorCond: map[string]any{"age": map[string]any{"$ge": 18}},
	},
	{
		name:       "or",
		condition:  mongory.M{"$or": mongory.A{mongory.M{"age": mongory.M{"$gte": 18}}, mongory.M{"status": "active"}}},
		connorCond: map[string]any{"$or": []any{map[string]any{"age": map[string]any{"$ge": 18}}, map[string]any{"status": "active"}}},
	},
	{
		name:       "nested_and",
		condition:  mongory.M{"profile.city": "Taipei", "status": "active", "age": mongory.M{"$lt": 40}},
		connorCond: map[string]any{"profile": map[string]any{"city": "Taipei"}, "status": "active", "age": map[string]any{"$lt": 40}},
	},
	{
		name:       "in_array_field",
		condition:  mongory.M{"tags": mongory.M{"$in": mongory.A{"go", "rust"}}},
		skipConnor: true, // connor $in checks scalar membership, not array intersection
	},
	{
		name:      "large_in_500",
		condition: mongory.M{"name": mongory.M{"$in": makeLargeInTargets(500)}},
	},
	{
		name:       "regex",
		condition:  mongory.M{"name": mongory.M{"$regex": "^user[a-f]$"}},
		lungoCond:  map[string]any{"name": primitive.Regex{Pattern: "^user[a-f]$"}},
		skipConnor: true, // connor has no $regex operator
	},
}

func BenchmarkCompetitors(b *testing.B) {
	maps, docs := benchData()
	plainMaps := make([]map[string]any, len(maps))
	for i, m := range maps {
		plainMaps[i] = toPlainMap(m)
	}

	for _, bc := range peerCases {
		m := mongory.MustCompile(bc.condition)
		plainCond := toPlainMap(bc.condition)

		b.Run(bc.name+"/mongory_go", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				n := 0
				for _, r := range maps {
					if m.Match(r) {
						n++
					}
				}
				_ = n
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchSize, "ns/record")
		})

		if !bc.skipLungo {
			lc := bc.lungoCond
			if lc == nil {
				lc = plainCond
			}
			lungoFilter, err := bsonkit.Convert(lc)
			if err != nil {
				b.Fatalf("%s: lungo convert: %v", bc.name, err)
			}
			b.Run(bc.name+"/lungo_mongokit", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					n := 0
					for _, doc := range docs {
						ok, err := mongokit.Match(doc, lungoFilter)
						if err != nil {
							b.Fatal(err)
						}
						if ok {
							n++
						}
					}
					_ = n
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchSize, "ns/record")
			})
		}

		if !bc.skipConnor {
			cc := bc.connorCond
			if cc == nil {
				cc = plainCond
			}
			// Verify connor supports this query before timing
			if _, err := connor.Match(cc, plainMaps[0]); err != nil {
				b.Logf("%s: skipping connor (%v)", bc.name, err)
			} else {
				b.Run(bc.name+"/connor", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						n := 0
						for _, r := range plainMaps {
							ok, _ := connor.Match(cc, r)
							if ok {
								n++
							}
						}
						_ = n
					}
					b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/benchSize, "ns/record")
				})
			}
		}
	}
}
