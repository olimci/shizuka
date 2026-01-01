package build

import (
	"context"
	"html/template"
	"runtime"
)

func defaultOptions() *Options {
	return &Options{
		context:          context.Background(),
		configPath:       "shizuka.toml",
		maxWorkers:       runtime.NumCPU(),
		Dev:              false,
		diagnosticSink:   NoopSink(),
		failOnLevel:      LevelError, // By default, only errors cause failure
		lenientErrors:    false,
		fallbackTemplate: nil,
	}
}

type Options struct {
	context    context.Context
	configPath string
	maxWorkers int
	Dev        bool

	// Diagnostic options
	diagnosticSink DiagnosticSink
	failOnLevel    DiagnosticLevel // Build fails if any diagnostic at or above this level
	lenientErrors  bool            // Treat certain errors as warnings (typically enabled in dev)

	// Fallback template for pages with missing templates
	fallbackTemplate *template.Template
}

func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// DiagnosticSink returns the configured diagnostic sink.
func (o *Options) DiagnosticSink() DiagnosticSink {
	return o.diagnosticSink
}

// FailOnLevel returns the minimum diagnostic level that causes build failure.
func (o *Options) FailOnLevel() DiagnosticLevel {
	return o.failOnLevel
}

// LenientErrors returns whether errors should be demoted to warnings.
func (o *Options) LenientErrors() bool {
	return o.lenientErrors
}

// FallbackTemplate returns the fallback template for pages with missing templates.
func (o *Options) FallbackTemplate() *template.Template {
	return o.fallbackTemplate
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

// WithDiagnosticSink sets the diagnostic sink for collecting build diagnostics.
func WithDiagnosticSink(sink DiagnosticSink) Option {
	return func(o *Options) {
		o.diagnosticSink = sink
	}
}

// WithFailOnLevel sets the minimum diagnostic level that causes build failure.
// Common values:
//   - LevelError (default): only errors cause failure
//   - LevelWarning: warnings and errors cause failure (strict mode)
//   - LevelInfo: even info messages cause failure (very strict, rarely used)
func WithFailOnLevel(level DiagnosticLevel) Option {
	return func(o *Options) {
		o.failOnLevel = level
	}
}

// WithFailOnWarnings is a convenience for WithFailOnLevel(LevelWarning).
func WithFailOnWarnings() Option {
	return WithFailOnLevel(LevelWarning)
}

// WithLenientErrors enables lenient mode where errors are demoted to warnings.
// This is typically used in dev mode to allow the build to continue despite errors.
func WithLenientErrors() Option {
	return func(o *Options) {
		o.lenientErrors = true
	}
}

// WithFallbackTemplate sets a fallback template to use when a page's template is missing.
// When set, pages with missing templates will be rendered using this template instead of
// causing an error. The template receives the full PageTemplate data including Meta.
func WithFallbackTemplate(tmpl *template.Template) Option {
	return func(o *Options) {
		o.fallbackTemplate = tmpl
	}
}
