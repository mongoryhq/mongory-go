package mongory

import (
	"fmt"
	"os"
	"testing"
)

func TestBasic(t *testing.T) {
	matcher, err := NewCMatcher(map[string]any{"key1": "hello"}, nil)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	result, err := matcher.Match(map[string]any{"key1": "hello"})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	fmt.Println("result", result)
}

func TestTrace(t *testing.T) {
	matcher, err := NewCMatcher(map[string]any{"key2": "hello"}, nil)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	err = matcher.EnableTrace()
	if err != nil {
		t.Fatalf("EnableTrace failed: %v", err)
	}
	result, err := matcher.Match(map[string]any{"key2": "hello"})
	fmt.Println("result", result)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	fmt.Println("result", result)
	result, err = matcher.Match(map[string]any{"key3": "world"})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	matcher.PrintTrace()
	matcher.DisableTrace()
	fmt.Println("result", result)
}

func TestExplain(t *testing.T) {
	matcher, err := NewCMatcher(map[string]any{
		"age": map[string]any{
			"$gt": 18,
			"$lt": 30,
		},
		"name": "John",
	}, nil)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	err = matcher.Explain()
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}
}

func TestInvalidCondition(t *testing.T) {
	_, err := NewCMatcher(map[string]any{
		"$and": "hello",
	}, nil)
	if err == nil {
		t.Fatalf("NewMatcher should fail for invalid $and condition")
	}
	fmt.Println("error", err)
}

func TestTeardown(t *testing.T) {
	fmt.Println("teardown")
}

func TestMain(m *testing.M) {
	Init()
	code := m.Run()
	Cleanup()
	os.Exit(code)
}
