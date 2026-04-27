package registry

import "testing"

func TestRegistryTypedAccessors(t *testing.T) {
	r := New()

	type keyType = K[int]
	const countKey keyType = "count"

	SetAs(r, countKey, 7)

	if got, ok := r.Get("count"); !ok || got.(int) != 7 {
		t.Fatalf("Get(count) = %v, %v; want 7, true", got, ok)
	}
	if got := GetAs(r, countKey); got != 7 {
		t.Fatalf("GetAs(count) = %d, want 7", got)
	}

	r.Set("count", "wrong")
	if got := GetAs(r, countKey); got != 0 {
		t.Fatalf("GetAs(wrong type) = %d, want 0", got)
	}
}
