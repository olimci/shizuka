package config

import (
	"context"
	"html/template"
	"maps"
	"runtime"

	"github.com/olimci/shizuka/pkg/events"
)

// DefaultOptions constructs an Options with default values.
func DefaultOptions() *Options {
	return &Options{
		Context:      context.Background(),
		ConfigPath:   "shizuka.toml",
		MaxWorkers:   runtime.NumCPU(),
		Dev:          false,
		EventHandler: new(events.NoopHandler),
	}
}

// Options represents the options for building a site.
type Options struct {
	Context    context.Context
	ConfigPath string
	OutputPath string
	SiteURL    string

	MaxWorkers int
	Dev        bool

	PageErrTemplates map[error]*template.Template
	ErrTemplate      *template.Template
	EventHandler     events.Handler
}

// WithContext sets the root context for building
func (o *Options) WithContext(ctx context.Context) *Options {
	o.Context = ctx
	return o
}

// WithConfig sets the path to the configuration file
func (o *Options) WithConfig(path string) *Options {
	o.ConfigPath = path
	return o
}

// WithOutput sets the path to the output directory, overriding config
func (o *Options) WithOutput(path string) *Options {
	o.OutputPath = path
	return o
}

// WithSiteURL sets the site base URL, overriding config.
func (o *Options) WithSiteURL(url string) *Options {
	o.SiteURL = url
	return o
}

// WithMaxWorkers sets the maximum number of workers to use for building
func (o *Options) WithMaxWorkers(n int) *Options {
	if n <= 0 {
		panic("max workers must be > 0")
	}

	o.MaxWorkers = n
	return o
}

// WithDev enables development mode
func (o *Options) WithDev() *Options {
	o.Dev = true
	return o
}

// WithEventHandler sets the event handler for building
func (o *Options) WithEventHandler(handler events.Handler) *Options {
	o.EventHandler = handler
	return o
}

// WithPageErrorTemplates sets the page error templates for building, nil corresponds to a catch-all template
func (o *Options) WithPageErrorTemplates(pages map[error]*template.Template) *Options {
	o.PageErrTemplates = maps.Clone(pages)
	return o
}

// WithErrTemplate sets the error template for building
func (o *Options) WithErrTemplate(tmpl *template.Template) *Options {
	o.ErrTemplate = tmpl
	return o
}
