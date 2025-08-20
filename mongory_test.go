package mongory

import (
	"fmt"
	"os"
	"testing"
)

func TestBasic(t *testing.T) {
	matcher, err := NewMatcher(map[string]any{"key": "hello"}, nil)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	result, err := matcher.Match(map[string]any{"key": "hello"})
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	fmt.Println("result", result)
}

func TestTrace(t *testing.T) {
	matcher, err := NewMatcher(map[string]any{"key": "hello"}, nil)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	result, err := matcher.Trace(map[string]any{"key": "hello"})
	if err != nil {
		t.Fatalf("Trace failed: %v", err)
	}
	fmt.Println("result", result)
	result, err = matcher.Trace(map[string]any{"key": "world"})
	if err != nil {
		t.Fatalf("Trace failed: %v", err)
	}
	fmt.Println("result", result)
}

func TestExplain(t *testing.T) {
	matcher, err := NewMatcher(map[string]any{
		"name": "John",
		"age": map[string]any{
			"$gt": 18,
			"$lt": 30,
		},
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
	_, err := NewMatcher(map[string]any{
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
