package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/mongoryhq/mongory-go"
)

type GCStat struct {
	Before runtime.MemStats
	After  runtime.MemStats
}

func printGC(stat GCStat) {
	created := int64(stat.After.Mallocs - stat.Before.Mallocs)
	freed := int64(stat.After.Frees - stat.Before.Frees)
	alive := int64(stat.After.HeapObjects - stat.Before.HeapObjects)
	fmt.Printf("Created: %d\n", created)
	fmt.Printf("Freed: %d\n", freed)
	fmt.Printf("Net alive: %d\n", alive)
}

func genRecords(size int) []mongory.M {
	r := rand.New(rand.NewPCG(1, 2))
	records := make([]mongory.M, size)
	for i := 0; i < size; i++ {
		var age any
		if r.IntN(2) == 0 {
			age = nil
		} else {
			age = r.IntN(100) + 1
		}
		status := "inactive"
		if r.IntN(2) == 0 {
			status = "active"
		}
		records[i] = mongory.M{
			"age":    age,
			"status": status,
		}
	}
	return records
}

func countSimpleQuery(records []mongory.M) int {
	n := 0
	for _, r := range records {
		v, ok := r["age"]
		if !ok {
			continue
		}
		if vv, ok := v.(int); ok && vv >= 18 {
			n++
		}
	}
	return n
}

func countComplexQuery(records []mongory.M) int {
	n := 0
	for _, r := range records {
		agev, hasAge := r["age"]
		statusv, hasStatus := r["status"]
		if !(hasAge && hasStatus) {
			continue
		}
		cond := false
		if vv, ok := agev.(int); ok {
			cond = vv >= 18
		}
		if cond || statusv == "active" {
			n++
		}
	}
	return n
}

func bench(title string, loops int, fn func()) {
	fmt.Printf("\n%s\n", title)
	old := debug.SetGCPercent(-1)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < loops; i++ {
		start := time.Now()
		fn()
		fmt.Println(time.Since(start))
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	_ = debug.SetGCPercent(old)
	printGC(GCStat{Before: before, After: after})
}

func main() {
	const size = 100_000
	const loops = 5

	fmt.Printf("Testing with %d records:\n", size)
	records := genRecords(size)

	expectedSimple := countSimpleQuery(records)
	expectedComplex := countComplexQuery(records)

	bench("Simple query (Plain Go)", loops, func() {
		_ = countSimpleQuery(records)
	})

	simpleMatcher := mongory.MustCompile(mongory.M{"age": mongory.M{"$gte": 18}})
	bench("Simple query (Mongory Matcher)", loops, func() {
		cnt := mongory.Count(records, simpleMatcher)
		if cnt != expectedSimple {
			panic(fmt.Sprintf("count mismatch: got %d want %d", cnt, expectedSimple))
		}
	})

	bench("Complex query (Plain Go)", loops, func() {
		_ = countComplexQuery(records)
	})

	complexMatcher := mongory.MustCompile(mongory.M{
		"$or": mongory.A{
			mongory.M{"age": mongory.M{"$gte": 18}},
			mongory.M{"status": "active"},
		},
	})
	bench("Complex query (Mongory Matcher)", loops, func() {
		cnt := mongory.Count(records, complexMatcher)
		if cnt != expectedComplex {
			panic(fmt.Sprintf("count mismatch: got %d want %d", cnt, expectedComplex))
		}
	})
}
