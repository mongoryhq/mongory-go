package mongory

import "testing"

func TestPing(t *testing.T) {
	if err := Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}
