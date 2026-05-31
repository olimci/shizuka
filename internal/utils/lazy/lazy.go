package lazy

import "sync"

func New[T any](get func() T) LazyValue[T] {
	return LazyValue[T]{
		get: get,
	}
}

func Must[T any](get func() (T, error)) LazyMust[T] {
	return LazyMust[T]{
		get: get,
	}
}

type LazyValue[T any] struct {
	once  sync.Once
	value T
	get   func() T
}

func (l *LazyValue[T]) Get() T {
	l.once.Do(func() {
		l.value = l.get()
	})
	return l.value
}

type LazyMust[T any] struct {
	once  sync.Once
	value T
	get   func() (T, error)
}

func (m *LazyMust[T]) Get() T {
	m.once.Do(func() {
		var err error
		m.value, err = m.get()
		if err != nil {
			panic(err)
		}
	})
	return m.value
}
