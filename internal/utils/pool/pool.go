package pool

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

var (
	ErrClosed = errors.New("work pool closed")
	ErrNilJob = errors.New("nil work job")
)

// Pool runs submitted work on a bounded set of goroutines.
type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc
	group  *errgroup.Group
	jobs   chan task
	done   chan struct{}

	submitMu sync.Mutex
	closed   bool

	waitErr error
}

type task func(context.Context) error

// New creates a bounded work pool.
func New(ctx context.Context, workers int) *Pool {
	if ctx == nil {
		ctx = context.Background()
	}
	if workers <= 0 {
		workers = 1
	}

	poolCtx, cancel := context.WithCancel(ctx)
	group, groupCtx := errgroup.WithContext(poolCtx)
	p := &Pool{
		ctx:    groupCtx,
		cancel: cancel,
		group:  group,
		jobs:   make(chan task, workers*4),
		done:   make(chan struct{}),
	}

	p.group.SetLimit(workers)
	go p.orchestrate()

	return p
}

func (p *Pool) orchestrate() {
	defer close(p.done)

	for task := range p.jobs {
		p.start(task)
	}
	p.waitErr = p.group.Wait()
}

func (p *Pool) start(task task) {
	p.group.Go(func() error {
		return task(p.ctx)
	})
}

// Go submits a fire-and-forget job. If the job returns an error, the pool is
// cancelled and Wait returns that error.
func (p *Pool) Go(fn func(context.Context) error) error {
	if fn == nil {
		return ErrNilJob
	}

	p.submitMu.Lock()
	defer p.submitMu.Unlock()

	if p.closed {
		return ErrClosed
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}

	select {
	case p.jobs <- task(fn):
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Close stops accepting work. Already accepted work is drained before Wait
// returns.
func (p *Pool) Close() {
	p.submitMu.Lock()
	defer p.submitMu.Unlock()

	if p.closed {
		return
	}
	close(p.jobs)
	p.closed = true
}

// Wait closes the pool, waits for accepted work to finish or be cancelled, and
// returns the first fatal worker error.
func (p *Pool) Wait() error {
	p.Close()
	<-p.done
	p.cancel()
	return p.waitErr
}
