package internal

import "testing"

func TestNew(t *testing.T) {
	t.Parallel()

	value := 42
	ptr := New(value)

	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}

	if *ptr != value {
		t.Fatalf("expected %d, got %d", value, *ptr)
	}
}
