package build

import (
	"context"
	"runtime"
)

func defaultOptions() *Options {
	return &Options{
		context:    context.Background(),
		configPath: "shizuka.toml",
		maxWorkers: runtime.NumCPU(),
		Dev:        false,
	}
}

type Options struct {
	context    context.Context
	configPath string
	maxWorkers int
	Dev        bool
}

func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type Option func(*Options)

func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.context = ctx
	}
}

func WithConfig(path string) Option {
	return func(o *Options) {
		o.configPath = path
	}
}

func WithMaxWorkers(n int) Option {
	return func(o *Options) {
		o.maxWorkers = n
	}
}

func WithDev() Option {
	return func(o *Options) {
		o.Dev = true
	}
}
