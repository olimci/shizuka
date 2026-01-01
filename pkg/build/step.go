package build

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/manifest"
)

// StepContext is the interface for the build step to interact with the build process.
type StepContext struct {
	Ctx      context.Context
	Manifest *manifest.Manifest
	Options  *Options
	StepID   string

	defers []Step
}

// Defer adds a step to be executed in the next phase of the build process
func (ctx *StepContext) Defer(step Step) {
	ctx.defers = append(ctx.defers, step)
}

// report is an internal method for reporting diagnostics
func (sc *StepContext) report(level DiagnosticLevel, source, message string, err error) {
	sc.Options.DiagnosticSink.Report(Diagnostic{
		Level:   level,
		StepID:  sc.StepID,
		Source:  source,
		Message: message,
		Err:     err,
	})
}

// Debug reports a debug message
func (sc *StepContext) Debug(source, message string) {
	sc.report(LevelDebug, source, message, nil)
}

// Debugf reports a debug message with a format string
func (sc *StepContext) Debugf(source, format string, args ...any) {
	sc.Debug(source, fmt.Sprintf(format, args...))
}

// Info reports an info-level message
func (sc *StepContext) Info(source, message string) {
	sc.report(LevelInfo, source, message, nil)
}

// Infof reports an info-level message with a format string
func (sc *StepContext) Infof(source, format string, args ...any) {
	sc.Info(source, fmt.Sprintf(format, args...))
}

// Warn reports a warning-level message
func (sc *StepContext) Warn(source, message string, err error) error {
	sc.report(LevelWarning, source, message, err)
	if sc.Options.LenientErrors || !sc.Options.FailOnWarn {
		return nil
	}

	if err != nil {
		return fmt.Errorf("%s: %s: %w", source, message, err)
	}

	return fmt.Errorf("%s: %s", source, message)
}

// Warnf reports a warning-level message with a format string
func (sc *StepContext) Warnf(source, format string, args ...any) error {
	return sc.Warn(source, fmt.Sprintf(format, args...), nil)
}

// Error reports an error-level message
func (sc *StepContext) Error(source, message string, err error) error {
	level := LevelError
	if sc.Options.LenientErrors {
		level = LevelWarning
	}

	sc.report(level, source, message, err)

	if sc.Options.LenientErrors {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %s: %w", source, message, err)
	}
	return fmt.Errorf("%s: %s", source, message)
}

// Errorf reports an error-level message with a format string
func (sc *StepContext) Errorf(source, format string, args ...any) error {
	return sc.Error(source, fmt.Sprintf(format, args...), nil)
}

// Step represents the DAG node for a build step
type Step struct {
	ID   string
	Deps []string
	Fn   func(*StepContext) error
}

// StepFunc creates a new Step with the given ID, function, and dependencies
func StepFunc(id string, fn func(*StepContext) error, deps ...string) Step {
	if deps == nil {
		deps = []string{}
	}

	return Step{
		ID:   id,
		Deps: deps,
		Fn:   fn,
	}
}
