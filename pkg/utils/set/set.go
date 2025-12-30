package set

func New[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]struct{})}
}

type Set[T comparable] struct {
	m map[T]struct{}
}

func (s *Set[T]) Add(v T) {
	s.m[v] = struct{}{}
}

func (s *Set[T]) Has(v T) bool {
	_, ok := s.m[v]
	return ok
}

func (s *Set[T]) Delete(v T) {
	delete(s.m, v)
}

func (s *Set[T]) Len() int {
	return len(s.m)
}

func (s *Set[T]) Values() []T {
	values := make([]T, 0, len(s.m))
	for k := range s.m {
		values = append(values, k)
	}
	return values
}

func (s *Set[T]) Clear() {
	clear(s.m)
}

func (s *Set[T]) Clone() *Set[T] {
	newSet := New[T]()
	for k := range s.m {
		newSet.Add(k)
	}
	return newSet
}
