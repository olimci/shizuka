package options

import (
	"context"
	"html/template"
	"maps"
	"runtime"

	"github.com/olimci/shizuka/pkg/registry"
)

type Option func(*Options)

func If(opt Option, enabled bool) Option {
	if !enabled {
		return nil
	}
	return opt
}

func Filter(values ...Option) []Option {
	filtered := make([]Option, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.Context = ctx
	}
}

func WithConfigPath(path string) Option {
	return func(o *Options) {
		o.ConfigPath = path
	}
}

func WithOutputPath(path string) Option {
	return func(o *Options) {
		o.OutputPath = path
	}
}

func WithSiteURL(url string) Option {
	return func(o *Options) {
		o.SiteURL = url
	}
}

func WithMaxWorkers(workers int) Option {
	return func(o *Options) {
		o.MaxWorkers = workers
	}
}

func WithDev(dev bool) Option {
	return func(o *Options) {
		o.Dev = dev
	}
}

func WithSyncWrites(sync bool) Option {
	return func(o *Options) {
		o.SyncWrites = sync
	}
}

func WithSkipOutputCleanup(skip bool) Option {
	return func(o *Options) {
		o.SkipOutputCleanup = skip
	}
}

func WithCacheRegistry(cache *registry.Registry) Option {
	return func(o *Options) {
		o.CacheRegistry = cache
	}
}

func WithChangedPaths(paths []string) Option {
	return func(o *Options) {
		o.ChangedPaths = append([]string(nil), paths...)
	}
}

func WithPageErrTemplates(templates map[error]*template.Template) Option {
	return func(o *Options) {
		o.PageErrTemplates = maps.Clone(templates)
	}
}

func WithErrTemplate(tmpl *template.Template) Option {
	return func(o *Options) {
		o.ErrTemplate = tmpl
	}
}

func DefaultOptions() *Options {
	return &Options{
		Context:           context.Background(),
		ConfigPath:        "shizuka.toml",
		MaxWorkers:        runtime.NumCPU(),
		SyncWrites:        false,
		SkipOutputCleanup: false,
		Dev:               false,
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

	SyncWrites        bool
	SkipOutputCleanup bool

	CacheRegistry *registry.Registry
	ChangedPaths  []string

	PageErrTemplates map[error]*template.Template
	ErrTemplate      *template.Template
}

func (o *Options) Apply(opts ...Option) *Options {
	if o == nil {
		o = DefaultOptions()
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(o)
	}
	return o
}
