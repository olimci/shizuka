package pool

import (
	"context"
	"sync"
)

type Future[T any] struct {
	done chan struct{}
	once sync.Once

	value T
	err   error
}

// Submit adds a job to the pool and returns a future for its result.
func Submit[T any](p *Pool, fn func(context.Context) (T, error)) *Future[T] {
	f := &Future[T]{
		done: make(chan struct{}),
	}

	if p == nil {
		f.finish(*new(T), ErrClosed)
		return f
	}

	err := p.Go(func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			f.finish(*new(T), err)
			return err
		}
		value, err := fn(ctx)
		f.finish(value, err)
		return err
	})
	if err != nil {
		f.finish(*new(T), err)
	}

	return f
}

func (f *Future[T]) Await() (T, error) {
	<-f.done
	return f.value, f.err
}

func (f *Future[T]) finish(value T, err error) {
	f.once.Do(func() {
		f.value = value
		f.err = err
		close(f.done)
	})
}
