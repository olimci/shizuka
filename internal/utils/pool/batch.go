package pool

import "context"

// Batch collects results from a set of jobs submitted to a Pool. Submit jobs
// with Go; call Wait once to retrieve them. Results are returned in submission
// order; jobs that returned an error are omitted from the slice and the first
// such error is returned.
type Batch[T any] struct {
	pool    *Pool
	futures []*Future[T]
}

func NewBatch[T any](p *Pool) *Batch[T] {
	return &Batch[T]{pool: p}
}

func (b *Batch[T]) Go(fn func(context.Context) (T, error)) {
	b.futures = append(b.futures, Submit(b.pool, fn))
}

func (b *Batch[T]) Wait() ([]T, error) {
	results := make([]T, 0, len(b.futures))
	var firstErr error
	for _, f := range b.futures {
		v, err := f.Await()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		results = append(results, v)
	}
	return results, firstErr
}
