package registry

import "fmt"

// K is a typed key.
type K[T any] string

type Setter interface {
	SetAny(string, any) bool
}

type Getter interface {
	GetAny(string) (any, bool)
}

type Deleter interface {
	DeleteAny(string) bool
}

func Get[T any](g Getter, k K[T]) T {
	if v, ok := GetOk(g, k); ok {
		return v
	}
	panic(fmt.Sprintf("unknown key %q", k))
}

func GetOk[T any](g Getter, k K[T]) (T, bool) {
	v, ok := g.GetAny(string(k))
	if !ok {
		var zero T
		return zero, false
	}
	if v == nil {
		var zero T
		return zero, true
	}
	if vt, ok := v.(T); ok {
		return vt, true
	}

	panic(fmt.Sprintf("value %q is not of type %T", v, *new(T)))
}

func Set[T any](s Setter, k K[T], v T) {
	if ok := s.SetAny(string(k), v); !ok {
		panic(fmt.Sprintf("failed to set key %q", k))
	}
}

func Delete[T any](d Deleter, k K[T]) {
	if ok := d.DeleteAny(string(k)); !ok {
		panic(fmt.Sprintf("failed to delete key %q", k))
	}
}
