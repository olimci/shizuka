package registry

import "testing"

func TestRegistryTypedGetSetDelete(t *testing.T) {
	type payload struct {
		Name string
	}
	key := K[payload]("payload")
	reg := New()

	Set(reg, key, payload{Name: "one"})
	if got := Get(reg, key); got.Name != "one" {
		t.Fatalf("payload = %#v, want one", got)
	}

	Delete(reg, key)
	if _, ok := GetOk(reg, key); ok {
		t.Fatal("deleted key still present")
	}
}

func TestRegistryLockScopesReadAndWriteAccess(t *testing.T) {
	key := K[string]("title")
	reg := New()
	Set(reg, key, "old")

	guard, scoped := reg.Lock(W(key))
	Set(scoped, key, "new")
	if got := Get(scoped, key); got != "new" {
		t.Fatalf("scoped value = %q, want new", got)
	}
	guard.Close()

	if got := Get(reg, key); got != "new" {
		t.Fatalf("registry value = %q, want new", got)
	}

	guard, scoped = reg.Lock(R(key))
	if ok := scoped.SetAny(string(key), "ignored"); ok {
		t.Fatal("read lock allowed SetAny")
	}
	guard.Close()
}

func TestRegistryOptionalReadOfMissingKey(t *testing.T) {
	key := K[string]("missing")
	reg := New()

	guard, scoped := reg.Lock(RX(key))
	defer guard.Close()

	if _, ok := GetOk(scoped, key); ok {
		t.Fatal("optional missing key returned present")
	}
}

func TestRegistryPanicsForUnknownRequiredRead(t *testing.T) {
	reg := New()
	defer assertPanic(t)
	reg.Lock(R(K[string]("missing")))
}

func TestRegistryPanicsForWrongType(t *testing.T) {
	reg := New()
	Set(reg, K[string]("value"), "text")

	defer assertPanic(t)
	Get[int](reg, K[int]("value"))
}

func assertPanic(t *testing.T) {
	t.Helper()
	if recover() == nil {
		t.Fatal("expected panic")
	}
}
