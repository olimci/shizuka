package logging

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"
)

func NewHandler(opts Options) slog.Handler {
	if opts.Out == nil {
		opts.Out = io.Discard
	}

	if opts.Err == nil {
		opts.Err = io.Discard
	}

	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}

	if opts.TimeFormat == "" {
		opts.TimeFormat = time.Kitchen
	}

	if opts.OutFormat == FormatUnset {
		opts.OutFormat = opts.Format
	}

	if opts.ErrFormat == FormatUnset {
		opts.ErrFormat = opts.Format
	}

	return &Handler{
		mu:     new(sync.Mutex),
		opts:   opts,
		attrs:  nil,
		groups: nil,
	}
}

type Handler struct {
	mu *sync.Mutex

	opts Options

	attrs  []groupedAttr
	groups []string
}

type groupedAttr struct {
	groups []string
	attr   slog.Attr
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	return h.write(r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	j := &Handler{
		mu:     h.mu,
		opts:   h.opts,
		attrs:  slices.Clone(h.attrs),
		groups: slices.Clone(h.groups),
	}

	for _, attr := range attrs {
		j.attrs = append(j.attrs, groupedAttr{
			groups: slices.Clone(h.groups),
			attr:   attr,
		})
	}

	return j
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	j := &Handler{
		mu:     h.mu,
		opts:   h.opts,
		attrs:  slices.Clone(h.attrs),
		groups: slices.Clone(h.groups),
	}

	j.groups = append(j.groups, name)

	return j
}
