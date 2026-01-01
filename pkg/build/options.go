package build

import (
	"context"
	"html/template"
	"runtime"
)

// defaultOptions constructs an Options with default values.
func defaultOptions() *Options {
	return &Options{
		Context:          context.Background(),
		ConfigPath:       "shizuka.toml",
		MaxWorkers:       runtime.NumCPU(),
		Dev:              false,
		DiagnosticSink:   NoopSink(),
		FailOnWarn:       false,
		LenientErrors:    false,
		FallbackTemplate: nil,
	}
}

// Options represents the options for building a site.
type Options struct {
	Context    context.Context
	ConfigPath string
	MaxWorkers int
	Dev        bool

	DiagnosticSink DiagnosticSink
	FailOnWarn     bool
	LenientErrors  bool

	FallbackTemplate *template.Template
}

// Apply applies a set of Option to the receiver.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	if o.FailOnWarn && o.LenientErrors {
		panic("build: cannot use FailOnWarn and LenientErrors together")
	}
	return o
}

// Option represents an option for building a site.
type Option func(*Options)

// WithContext attatches a context to be used by the build process
func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.Context = ctx
	}
}

// WithConfig sets the path to the configuration file
func WithConfig(path string) Option {
	return func(o *Options) {
		o.ConfigPath = path
	}
}

// WithMaxWorkers sets the maximum number of workers to use during the build process
func WithMaxWorkers(n int) Option {
	return func(o *Options) {
		o.MaxWorkers = n
	}
}

// WithDev enables development mode
func WithDev() Option {
	return func(o *Options) {
		o.Dev = true
	}
}

// WithDiagnosticSink sets the diagnostic sink to be used during the build process
func WithDiagnosticSink(sink DiagnosticSink) Option {
	return func(o *Options) {
		o.DiagnosticSink = sink
	}
}

// WithFailOnWarn enables fail on warning mode
func WithFailOnWarn() Option {
	return func(o *Options) {
		o.FailOnWarn = true
	}
}

// WithLenientErrors enables lenient errors mode
func WithLenientErrors() Option {
	return func(o *Options) {
		o.LenientErrors = true
	}
}

// WithFallbackTemplate sets the fallback template to be used during the build process
func WithFallbackTemplate(tmpl *template.Template) Option {
	return func(o *Options) {
		o.FallbackTemplate = tmpl
	}
}
