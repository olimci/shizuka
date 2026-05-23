package pool

import (
	"context"
	"sync"
)

type Stream[T any] struct {
	pool *Pool

	results chan T
	done    chan struct{}

	wg sync.WaitGroup

	mu        sync.Mutex
	expected  int
	submitted int
	sealed    bool
}

type Result[T any] struct {
	Value T
	Err   error
}

// NewStream creates a result stream for an already-known number of jobs.
func NewStream[T any](p *Pool, jobs int) *Stream[T] {
	if jobs < 0 {
		jobs = 0
	}

	s := &Stream[T]{
		pool:     p,
		results:  make(chan T, jobs),
		done:     make(chan struct{}),
		expected: jobs,
	}
	if jobs == 0 {
		close(s.results)
		close(s.done)
	}
	return s
}

func (s *Stream[T]) Go(fn func(context.Context) (T, error)) error {
	if s.pool == nil {
		return ErrClosed
	}

	s.mu.Lock()
	if s.submitted >= s.expected {
		s.mu.Unlock()
		return ErrStreamFull
	}
	s.submitted++
	s.wg.Add(1)
	shouldSeal := s.submitted == s.expected && !s.sealed
	if shouldSeal {
		s.sealed = true
	}
	s.mu.Unlock()

	err := s.pool.Go(func(ctx context.Context) error {
		defer s.wg.Done()

		if err := ctx.Err(); err != nil {
			return err
		}
		value, err := fn(ctx)
		if err != nil {
			return err
		}

		select {
		case s.results <- value:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if err != nil {
		s.wg.Done()
	}
	if shouldSeal {
		go s.closeWhenDone()
	}
	return err
}

func (s *Stream[T]) Results() <-chan T {
	return s.results
}

func (s *Stream[T]) Await() error {
	s.mu.Lock()
	incomplete := s.submitted != s.expected
	done := s.done
	s.mu.Unlock()

	if incomplete {
		return ErrStreamIncomplete
	}

	<-done
	return nil
}

func (s *Stream[T]) closeWhenDone() {
	s.wg.Wait()
	close(s.results)
	close(s.done)
}
