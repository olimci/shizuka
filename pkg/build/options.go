package build

import (
	"context"
	"html/template"
	"runtime"
)

// defaultOptions constructs an Options with default values.
func defaultOptions() *Options {
	return &Options{
		Context:            context.Background(),
		ConfigPath:         "shizuka.toml",
		MaxWorkers:         runtime.NumCPU(),
		Dev:                false,
		DiagnosticSink:     NoopSink(),
		FailOnWarn:         false,
		LenientErrors:      false,
		ErrPages:           nil,
		DefaultErrPage:     nil,
		DevFailureTemplate: nil,
		DevFailureTarget:   "",
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

	ErrPages       map[error]*template.Template
	DefaultErrPage *template.Template

	DevFailureTemplate *template.Template
	DevFailureTarget   string
}

// Apply applies a set of Option to the receiver.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	if o.Dev && !o.FailOnWarn {
		o.LenientErrors = true
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

// WithErrPages sets a map of error -> template to render pages for matching errors in dev mode,
// plus an optional default template used when no error matches.
func WithErrPages(pages map[error]*template.Template, defaultTemplate *template.Template) Option {
	return func(o *Options) {
		o.DefaultErrPage = defaultTemplate
		if pages == nil {
			return
		}

		if o.ErrPages == nil {
			o.ErrPages = make(map[error]*template.Template, len(pages))
		} else {
			for k := range o.ErrPages {
				delete(o.ErrPages, k)
			}
		}

		for match, tmpl := range pages {
			o.ErrPages[match] = tmpl
		}
	}
}

// WithDevFailurePage sets a template to render to a single page (default index.html)
// if the build fails after artefacts were written (e.g. strict diagnostics).
func WithDevFailurePage(tmpl *template.Template) Option {
	return func(o *Options) {
		o.DevFailureTemplate = tmpl
	}
}

// WithDevFailureTarget sets the target path for the dev failure page (defaults to index.html).
func WithDevFailureTarget(target string) Option {
	return func(o *Options) {
		o.DevFailureTarget = target
	}
}
