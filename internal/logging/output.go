package logging

import (
	"io"
	"log/slog"
)

type output struct {
	w      io.Writer
	tty    bool
	format Format
}

func (h *Handler) write(r slog.Record) error {
	outputs := h.outputs(r.Level)

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, output := range outputs {
		line, err := h.formatForOutput(r, output)
		if err != nil {
			return err
		}

		if _, err := output.w.Write(line); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) outputs(level slog.Level) []output {
	out := output{
		w:      h.opts.Out,
		tty:    h.opts.OutTTY,
		format: h.opts.OutFormat,
	}

	err := output{
		w:      h.opts.Err,
		tty:    h.opts.ErrTTY,
		format: h.opts.ErrFormat,
	}

	if level < slog.LevelWarn {
		return []output{out}
	}

	switch h.opts.ErrorOutput {
	case ErrorOutputAuto:
		if h.opts.OutAndErrSame || h.opts.Out == h.opts.Err || (h.opts.OutTTY && h.opts.ErrTTY) {
			return []output{out}
		}

		// When stdout is redirected to a file but stderr is still a terminal
		// (e.g. `shizuka build --debug > debug.log`), the file is the source
		// of truth — write warn+ there too, but also mirror to the terminal
		// so the user still sees them.
		if !h.opts.OutTTY && h.opts.ErrTTY {
			return []output{out, err}
		}

		return []output{err}

	case ErrorOutputOut:
		return []output{out}

	case ErrorOutputBoth:
		if h.opts.OutAndErrSame || h.opts.Out == h.opts.Err {
			return []output{out}
		}

		return []output{err, out}

	case ErrorOutputErr:
		fallthrough

	default:
		return []output{err}
	}
}

func (h *Handler) formatForOutput(r slog.Record, output output) ([]byte, error) {
	format := h.resolveFormat(output.format, output.tty)

	switch format {
	case FormatJSON:
		return h.formatJSON(r)

	case FormatPlain:
		return h.format(r, false)

	case FormatPretty:
		return h.format(r, true)

	default:
		return h.format(r, false)
	}
}

func (h *Handler) resolveFormat(format Format, tty bool) Format {
	if format == FormatUnset {
		format = h.opts.Format
	}

	switch format {
	case FormatJSON:
		return FormatJSON

	case FormatPlain:
		return FormatPlain

	case FormatPretty:
		return FormatPretty

	case FormatAuto, FormatUnset:
		if tty {
			return FormatPretty
		}

		return FormatPlain

	default:
		return FormatPlain
	}
}
