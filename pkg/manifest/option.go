package manifest

import (
	"context"
	"runtime"
)

// defaultOptions returns the default options
func defaultOptions() *options {
	return &options{
		BuildDir:        "dist",
		Context:         context.Background(),
		maxWorkers:      runtime.NumCPU(),
		ignoreConflicts: true,
	}
}

// options represents the options for the manifest
type options struct {
	BuildDir        string
	Context         context.Context
	maxWorkers      int
	ignoreConflicts bool
}

// apply applies the given options to the options struct
func (o *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// Option represents an option for the manifest
type Option func(*options)

// WithBuildDir sets the build directory
func WithBuildDir(dir string) Option {
	return func(o *options) {
		o.BuildDir = dir
	}
}

// WithContext attatches a context
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.Context = ctx
	}
}

// WithMaxWorkers sets the maximum number of workers when building
func WithMaxWorkers(n int) Option {
	return func(o *options) {
		o.maxWorkers = n
	}
}

// IgnoreConflicts ignores conflicting claims when building
func IgnoreConflicts() Option {
	return func(o *options) {
		o.ignoreConflicts = true
	}
}
