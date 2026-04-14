package config

import (
	"context"
	"html/template"
	"runtime"
)

// DefaultOptions constructs an Options with default values.
func DefaultOptions() *Options {
	return &Options{
		Context:    context.Background(),
		ConfigPath: "shizuka.toml",
		MaxWorkers: runtime.NumCPU(),
		Dev:        false,
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
}
