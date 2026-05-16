package internal

import "testing"

func TestNew_returnsPointerToValue(t *testing.T) {
	t.Parallel()

	input := 123

	got := New(input)
	if got == nil {
		t.Fatalf("New(%d) returned nil", input)
	}

	if *got != input {
		t.Fatalf("New(%d) = %d", input, *got)
	}
}
