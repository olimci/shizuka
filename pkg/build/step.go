package build

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/manifest"
)

type StepContext struct {
	Ctx      context.Context
	Manifest *manifest.Manifest
	Options  *Options
	StepID   string // The ID of the step, for diagnostic attribution

	defers []Step
}

func (ctx *StepContext) Defer(step Step) {
	ctx.defers = append(ctx.defers, step)
}

// report is the internal method for reporting diagnostics.
func (sc *StepContext) report(level DiagnosticLevel, source, message string, err error) {
	sc.Options.DiagnosticSink().Report(Diagnostic{
		Level:   level,
		StepID:  sc.StepID,
		Source:  source,
		Message: message,
		Err:     err,
	})
}

// Debug reports a debug-level diagnostic. For verbose troubleshooting info.
func (sc *StepContext) Debug(source, message string) {
	sc.report(LevelDebug, source, message, nil)
}

// Debugf is a formatted version of Debug.
func (sc *StepContext) Debugf(source, format string, args ...any) {
	sc.Debug(source, fmt.Sprintf(format, args...))
}

// Info reports an info-level diagnostic. For general progress information.
func (sc *StepContext) Info(source, message string) {
	sc.report(LevelInfo, source, message, nil)
}

// Infof is a formatted version of Info.
func (sc *StepContext) Infof(source, format string, args ...any) {
	sc.Info(source, fmt.Sprintf(format, args...))
}

// Warn reports a warning diagnostic. Something went wrong but build continues.
func (sc *StepContext) Warn(source, message string, err error) {
	sc.report(LevelWarning, source, message, err)
}

// Warnf is a formatted version of Warn.
func (sc *StepContext) Warnf(source, format string, args ...any) {
	sc.Warn(source, fmt.Sprintf(format, args...), nil)
}

// Error reports an error diagnostic. Serious issue.
// Returns nil if LenientErrors is enabled (error demoted to warning).
// Returns an error if the diagnostic should be fatal.
func (sc *StepContext) Error(source, message string, err error) error {
	level := LevelError
	if sc.Options.LenientErrors() {
		level = LevelWarning
	}

	sc.report(level, source, message, err)

	if sc.Options.LenientErrors() {
		return nil // Demoted to warning, continue build
	}
	if err != nil {
		return fmt.Errorf("%s: %s: %w", source, message, err)
	}
	return fmt.Errorf("%s: %s", source, message)
}

// Errorf is a formatted version of Error.
func (sc *StepContext) Errorf(source, format string, args ...any) error {
	return sc.Error(source, fmt.Sprintf(format, args...), nil)
}

type Step struct {
	ID   string
	Deps []string
	Func func(*StepContext) error
}

func StepFunc(id string, fn func(*StepContext) error, deps ...string) Step {
	if deps == nil {
		deps = []string{}
	}

	return Step{
		ID:   id,
		Deps: deps,
		Func: fn,
	}
}
