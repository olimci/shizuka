package scaffold

import (
	"io"
	"os"
)

func defaultOptions() *options {
	return &options{
		output: os.Stdout,
		quiet:  false,
	}
}

type options struct {
	output io.Writer
	quiet  bool
}

func (o *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(o)
	}

	return o
}

type Option func(*options)

func WithOutput(w io.Writer) Option {
	return func(o *options) {
		o.output = w
	}
}

func WithQuiet(quiet bool) Option {
	return func(o *options) {
		o.quiet = quiet
	}
}
