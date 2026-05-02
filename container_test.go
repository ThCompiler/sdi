package sdi

import "testing"

func TestContainerRegisterAndResolve(t *testing.T) {
	t.Parallel()

	container := New()
	container.Register("answer", 42)

	instance, err := container.Resolve("answer")
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}

	value, ok := instance.(int)
	if !ok {
		t.Fatalf("resolved value has unexpected type %T", instance)
	}

	if value != 42 {
		t.Fatalf("resolved value = %d, want 42", value)
	}
}

func TestContainerResolveMissing(t *testing.T) {
	t.Parallel()

	container := New()

	if _, err := container.Resolve("missing"); err == nil {
		t.Fatal("resolve should fail for missing dependency")
	}
}
