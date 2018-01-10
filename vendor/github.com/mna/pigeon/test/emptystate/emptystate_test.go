package emptystate

import "testing"

func TestEmptyState(t *testing.T) {
	got, err := Parse("", []byte("abcde"))
	if err != nil {
		t.Fatal(err)
	}
	state, ok := got.(storeDict)
	if !ok {
		t.Fatalf("expected map, got %T: %[1]v", got)
	}
	if len(state) != 0 {
		t.Fatalf("want empty state, got %v", state)
	}
}
