package main

import (
	"fmt"
	"mongory/pkg/mongory"
	"testing"
)

func TestBasic(t *testing.T) {
	matcher := mongory.NewMatcher(map[string]any{"key": "hello"}, nil)
	result := matcher.Match(map[string]any{"key": "hello"})
	fmt.Println("result", result)
}

func TestTrace(t *testing.T) {
	matcher := mongory.NewMatcher(map[string]any{"key": "hello"}, nil)
	result := matcher.Trace(map[string]any{"key": "hello"})
	fmt.Println("result", result)
	result = matcher.Trace(map[string]any{"key": "world"})
	fmt.Println("result", result)
}

func TestExplain(t *testing.T) {
	matcher := mongory.NewMatcher(map[string]any{
		"name": "John",
		"age": map[string]any{
			"$gt": 18,
			"$lt": 30,
		},
	}, nil)
	matcher.Explain()
}

func TestTeardown(t *testing.T) {
	fmt.Println("teardown")
	mongory.Cleanup()
}
