package options

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/utils/urlutil"
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

func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) {
		o.Logger = logger
	}
}

func WithConfigPath(path string) Option {
	return func(o *Options) {
		if path == "" {
			o.ConfigPath = ""
			return
		}
		o.ConfigPath = filepath.Clean(path)
	}
}

func WithOutputPath(path string) Option {
	return func(o *Options) {
		if o.OutputPathInternal {
			panic("options: output path is owned internally")
		}
		if path == "" {
			o.OutputPath = ""
		} else {
			o.OutputPath = filepath.Clean(path)
		}
		o.OutputPathInternal = false
	}
}

func WithSiteURL(url string) Option {
	return func(o *Options) {
		if o.siteURLInternal {
			panic("options: site url is owned internally")
		}
		if url == "" {
			o.SiteURL = ""
		} else {
			cleaned, err := urlutil.ValidURL(url)
			if err != nil {
				panic("options: invalid site url: " + err.Error())
			}
			o.SiteURL = cleaned
		}
		o.siteURLInternal = false
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

func WithCache(cache *registry.Registry) Option {
	return func(o *Options) {
		if o.cacheInternal {
			panic("options: cache registry is owned internally")
		}
		o.CacheRegistry = cache
		o.cacheInternal = false
	}
}

func WithChanges(paths []string) Option {
	return func(o *Options) {
		if o.changesInternal {
			panic("options: changed paths are owned internally")
		}
		o.ChangedPaths = CleanChangedPaths(paths)
		o.changesInternal = false
	}
}

func WithInternalOutputPath(path string) Option {
	return func(o *Options) {
		if o.OutputPath != "" && !o.OutputPathInternal {
			panic("options: output path was already set by caller")
		}
		if path == "" {
			o.OutputPath = ""
		} else {
			o.OutputPath = filepath.Clean(path)
		}
		o.OutputPathInternal = true
	}
}

func WithInternalSiteURL(url string) Option {
	return func(o *Options) {
		if o.SiteURL != "" && !o.siteURLInternal {
			panic("options: site url was already set by caller")
		}
		if url == "" {
			o.SiteURL = ""
		} else {
			cleaned, err := urlutil.ValidURL(url)
			if err != nil {
				panic("options: invalid site url: " + err.Error())
			}
			o.SiteURL = cleaned
		}
		o.siteURLInternal = true
	}
}

func WithInternalCache(cache *registry.Registry) Option {
	return func(o *Options) {
		if o.CacheRegistry != nil && !o.cacheInternal {
			panic("options: cache registry was already set by caller")
		}
		o.CacheRegistry = cache
		o.cacheInternal = true
	}
}

func WithInternalChanges(paths []string) Option {
	return func(o *Options) {
		if o.ChangedPaths != nil && !o.changesInternal {
			panic("options: changed paths were already set by caller")
		}
		o.ChangedPaths = append([]string(nil), paths...)
		o.changesInternal = true
	}
}

func DefaultOptions() *Options {
	return &Options{
		Context:    context.Background(),
		Logger:     slog.Default(),
		ConfigPath: "shizuka.jsonc",
		MaxWorkers: runtime.NumCPU(),
		SyncWrites: true, // should this be true?
		Dev:        false,
	}
}

// Options represents the options for building a site.
type Options struct {
	// Core
	Context context.Context
	Logger  *slog.Logger

	// Config overrides
	ConfigPath string
	OutputPath string
	SiteURL    string

	// Dev-mode stuff
	Dev bool

	// Runtime options
	MaxWorkers int
	SyncWrites bool

	// Cache Options
	CacheRegistry *registry.Registry
	ChangedPaths  []string

	// Internal options
	OutputPathInternal bool
	siteURLInternal    bool
	cacheInternal      bool
	changesInternal    bool
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

func CleanChangedPaths(paths []string) []string {
	if paths == nil {
		return nil
	}

	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		if raw == "" {
			continue
		}

		abs, err := filepath.Abs(raw)
		if err != nil {
			out = append(out, filepath.Clean(raw))
			continue
		}
		out = append(out, filepath.Clean(abs))
	}

	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) == 0 {
		return []string{}
	}
	return out
}
