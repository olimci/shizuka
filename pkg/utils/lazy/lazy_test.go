package lazy

import (
	"errors"
	"testing"
)

func TestLazyValueOnlyEvaluatesOnce(t *testing.T) {
	calls := 0
	value := New(func() int {
		calls++
		return 42
	})

	if got := value.Get(); got != 42 {
		t.Fatalf("first Get() = %d, want 42", got)
	}
	if got := value.Get(); got != 42 {
		t.Fatalf("second Get() = %d, want 42", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestLazyMustPanicsOnError(t *testing.T) {
	value := Must(func() (int, error) {
		return 0, errors.New("boom")
	})

	defer func() {
		if recover() == nil {
			t.Fatal("Get() did not panic")
		}
	}()

	_ = value.Get()
}
