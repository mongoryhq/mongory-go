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
	alive := int64((stat.After.HeapObjects) - (stat.Before.HeapObjects))
	fmt.Printf("Created: %d\n", created)
	fmt.Printf("Freed: %d\n", freed)
	fmt.Printf("Net alive: %d\n", alive)
}

func genRecords(size int) []map[string]any {
	records := make([]map[string]any, size)
	for i := 0; i < size; i++ {
		var age any
		if rand.IntN(2) == 0 { // nil or number
			age = nil
		} else {
			age = rand.IntN(100) + 1 // 1..100
		}
		status := "inactive"
		if rand.IntN(2) == 0 {
			status = "active"
		}
		records[i] = map[string]any{
			"age":    age,
			"status": status,
		}
	}
	return records
}

func countSimpleQuery(records []map[string]any) int {
	n := 0
	for _, r := range records {
		v, ok := r["age"]
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case int:
			if vv >= 18 {
				n++
			}
		case int64:
			if vv >= 18 {
				n++
			}
		case float64:
			if vv >= 18 {
				n++
			}
		}
	}
	return n
}

func countComplexQuery(records []map[string]any) int {
	n := 0
	for _, r := range records {
		agev, hasAge := r["age"]
		statusv, hasStatus := r["status"]
		if !(hasAge && hasStatus) {
			continue
		}
		cond := false
		switch vv := agev.(type) {
		case int:
			cond = vv >= 18
		case int64:
			cond = vv >= 18
		case float64:
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
		elapsed := time.Since(start)
		fmt.Println(elapsed)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	_ = debug.SetGCPercent(old)
	stat := GCStat{Before: before, After: after}
	printGC(stat)
}

func main() {
	// Ensure native runtime is initialized
	mongory.Init()
	defer mongory.Cleanup()

	size := 100_000
	loops := 5

	fmt.Printf("Testing with %d records:\n", size)
	records := genRecords(size)

	expectedSimple := countSimpleQuery(records)
	expectedComplex := countComplexQuery(records)

	// Plain Go simple query
	bench("Simple query (Plain Go)", loops, func() {
		_ = countSimpleQuery(records)
	})

	// Matcher simple query (reuse matcher across runs)
	matcherSimple, err := mongory.NewMatcher(map[string]any{
		"age": map[string]any{"$gte": 18},
	}, nil)
	if err != nil {
		panic(err)
	}
	bench("Simple query (Mongory Matcher)", loops, func() {
		cnt := 0
		for i := range records {
			ok, err := matcherSimple.Match(records[i])
			if err != nil {
				panic(err)
			}
			if ok {
				cnt++
			}
		}
		if cnt != expectedSimple {
			panic(fmt.Sprintf("count mismatch: got %d want %d", cnt, expectedSimple))
		}
	})

	// Plain Go complex query
	bench("Complex query (Plain Go)", loops, func() {
		_ = countComplexQuery(records)
	})

	// Matcher complex query: $or of age>=18 or status==active (reuse matcher)
	matcherComplex, err := mongory.NewMatcher(map[string]any{
		"$or": []any{
			map[string]any{"age": map[string]any{"$gte": 18}},
			map[string]any{"status": "active"},
		},
	}, nil)
	if err != nil {
		panic(err)
	}
	bench("Complex query (Mongory Matcher)", loops, func() {
		cnt := 0
		for i := range records {
			ok, err := matcherComplex.Match(records[i])
			if err != nil {
				panic(err)
			}
			if ok {
				cnt++
			}
		}
		if cnt != expectedComplex {
			panic(fmt.Sprintf("count mismatch: got %d want %d", cnt, expectedComplex))
		}
	})

}
