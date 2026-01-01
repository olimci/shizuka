package set

// New creates a new set
func New[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]struct{})}
}

// Set represents a set of elements of type T
type Set[T comparable] struct {
	m map[T]struct{}
}

// Add adds an element to the set
func (s *Set[T]) Add(t T) {
	s.m[t] = struct{}{}
}

// Has checks if the set contains an element
func (s *Set[T]) Has(t T) bool {
	_, ok := s.m[t]
	return ok
}

// Delete removes an element from the set
func (s *Set[T]) Delete(t T) {
	delete(s.m, t)
}

// Len returns the number of elements in the set
func (s *Set[T]) Len() int {
	return len(s.m)
}

// Values returns a slice of all elements in the set
func (s *Set[T]) Values() []T {
	values := make([]T, 0, len(s.m))
	for k := range s.m {
		values = append(values, k)
	}
	return values
}

// Clear removes all elements from the set
func (s *Set[T]) Clear() {
	clear(s.m)
}

// Clone creates a copy of the set
func (s *Set[T]) Clone() *Set[T] {
	newSet := New[T]()
	for k := range s.m {
		newSet.Add(k)
	}
	return newSet
}
