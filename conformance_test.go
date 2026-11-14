package mongory_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mongoryhq/mongory-go"
)

// Cross-language conformance tests driven by mongory-spec fixtures.
// Point MONGORY_SPEC_DIR at a mongory-spec checkout to enable:
//
//	MONGORY_SPEC_DIR=../mongory-spec go test -run TestConformance ./...

// conformanceDirs lists the fixture groups the pure Go engine supports so far.
var conformanceDirs = []string{"field"}

type specSuite struct {
	Description string         `json:"description"`
	Condition   map[string]any `json:"condition"`
	Error       bool           `json:"error"`
	Records     []struct {
		Data     any  `json:"data"`
		Expected bool `json:"expected"`
	} `json:"records"`
}

func TestConformance(t *testing.T) {
	specDir := os.Getenv("MONGORY_SPEC_DIR")
	if specDir == "" {
		t.Skip("MONGORY_SPEC_DIR not set")
	}
	for _, dir := range conformanceDirs {
		files, err := filepath.Glob(filepath.Join(specDir, "fixtures", dir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatalf("no fixtures under %s/fixtures/%s", specDir, dir)
		}
		for _, file := range files {
			runSpecFile(t, file)
		}
	}
}

func runSpecFile(t *testing.T, file string) {
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keep 30 and 30.0 distinct
	var suites []specSuite
	if err := dec.Decode(&suites); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	for _, s := range suites {
		t.Run(filepath.Base(file)+"/"+s.Description, func(t *testing.T) {
			q, err := mongory.New(s.Condition)
			if s.Error {
				if err == nil {
					t.Fatalf("expected error for condition %v", s.Condition)
				}
				return
			}
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			for _, r := range s.Records {
				if got := q.Match(r.Data); got != r.Expected {
					t.Errorf("Match(%v) = %v, want %v", r.Data, got, r.Expected)
				}
			}
		})
	}
}
