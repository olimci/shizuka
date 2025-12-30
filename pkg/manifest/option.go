package manifest

import (
	"context"
	"runtime"
)

func defaultOptions() *options {
	return &options{
		BuildDir:        "dist",
		Context:         context.Background(),
		maxWorkers:      runtime.NumCPU(),
		ignoreConflicts: true,
	}
}

type options struct {
	BuildDir        string
	Context         context.Context
	maxWorkers      int
	ignoreConflicts bool
}

func (o *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(o)
	}

	return o
}

type Option func(*options)

func WithBuildDir(dir string) Option {
	return func(o *options) {
		o.BuildDir = dir
	}
}

func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.Context = ctx
	}
}

func WithMaxWorkers(n int) Option {
	return func(o *options) {
		o.maxWorkers = n
	}
}

func IgnoreConflicts() Option {
	return func(o *options) {
		o.ignoreConflicts = true
	}
}
