package logging

import (
	"io"
	"log/slog"
)

type Format int

const (
	FormatUnset Format = iota
	FormatAuto
	FormatPlain
	FormatPretty
	FormatJSON
)

type ErrorOutput int

const (
	ErrorOutputAuto ErrorOutput = iota
	ErrorOutputErr
	ErrorOutputOut
	ErrorOutputBoth
)

type Options struct {
	Out io.Writer
	Err io.Writer

	OutTTY bool
	ErrTTY bool

	OutAndErrSame bool

	Color bool

	// Format is the default format for both streams.
	Format Format

	// OutFormat and ErrFormat override Format for each stream.
	OutFormat Format
	ErrFormat Format

	// ErrorOutput controls where warn and error records are written.
	ErrorOutput ErrorOutput

	Level      slog.Leveler
	AddSource  bool
	TimeFormat string
}
