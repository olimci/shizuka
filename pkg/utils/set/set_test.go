package set

import (
	"slices"
	"testing"
)

func TestSetOperations(t *testing.T) {
	s := New[string]()
	s.Add("a")
	s.Add("b")

	if !s.Has("a") {
		t.Fatal("Has(a) = false, want true")
	}
	if had := s.HasAdd("b"); !had {
		t.Fatal("HasAdd(existing) = false, want true")
	}
	if had := s.HasAdd("c"); had {
		t.Fatal("HasAdd(new) = true, want false")
	}

	values := s.Values()
	slices.Sort(values)
	if !slices.Equal(values, []string{"a", "b", "c"}) {
		t.Fatalf("Values() = %#v, want %#v", values, []string{"a", "b", "c"})
	}

	clone := s.Clone()
	clone.Delete("a")
	if !s.Has("a") {
		t.Fatal("original set mutated after clone delete")
	}

	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("Len() after Clear() = %d, want 0", s.Len())
	}
}

func TestFromSliceDeduplicates(t *testing.T) {
	s := FromSlice([]int{1, 2, 2, 3})
	if s.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", s.Len())
	}
}
