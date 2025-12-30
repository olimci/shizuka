package manifest

type K[T any] string

type Setter interface {
	Set(k string, v any)
}

type Getter interface {
	Get(k string) (any, bool)
}

type GetSetter interface {
	Getter
	Setter
}

func Set[T any](s Setter, k K[T], v T) {
	s.Set(string(k), v)
}

func Get[T any](g Getter, k K[T]) (T, bool) {
	if v, ok := g.Get(string(k)); ok {
		if vt, ok := v.(T); ok {
			return vt, true
		}
	}
	return *new(T), false
}

func GetUnsafe[T any](g Getter, k K[T]) T {
	if v, ok := g.Get(string(k)); ok {
		if vt, ok := v.(T); ok {
			return vt
		}
	}
	return *new(T)
}
