package steps

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/olimci/shizuka/pkg/events"
	"github.com/olimci/shizuka/pkg/manifest"
)

type StepID struct {
	Owner string
	Name  string
	Sub   string
}

func (s StepID) String() string {
	if s.Sub == "" {
		return fmt.Sprintf("%s:%s", s.Owner, s.Name)
	}
	return fmt.Sprintf("%s:%s:%s", s.Owner, s.Name, s.Sub)
}

// Step represents the DAG node for a build step.
type Step struct {
	ID     StepID
	Deps   []StepID
	Reads  []string
	Writes []string
	Fn     func(*StepContext) error
}

// StepFunc creates a new Step with the given ID and function.
func StepFunc(id StepID, fn func(*StepContext) error) Step {
	return Step{
		ID: id,
		Fn: fn,
	}
}

// WithReads returns a copy of the step with read resources attached.
func (s Step) WithReads(reads ...string) Step {
	s.Reads = append([]string(nil), reads...)
	return s
}

// WithWrites returns a copy of the step with write resources attached.
func (s Step) WithWrites(writes ...string) Step {
	s.Writes = append([]string(nil), writes...)
	return s
}

// WithDeps returns a copy of the step with dependencies attached.
func (s Step) WithDeps(deps ...StepID) Step {
	s.Deps = append(s.Deps, deps...)
	return s
}

// StepContext is the interface for the build step to interact with the build process.
type StepContext struct {
	Ctx        context.Context
	Manifest   *manifest.Manifest
	SourceFS   fs.FS
	SourceRoot string

	eventHandler events.Handler
}

func NewStepContext(ctx context.Context, man *manifest.Manifest, sourceFS fs.FS, sourceRoot string, handler events.Handler) StepContext {
	return StepContext{
		Ctx:          ctx,
		Manifest:     man,
		SourceFS:     sourceFS,
		SourceRoot:   sourceRoot,
		eventHandler: handler,
	}
}

func (sc *StepContext) event(level events.Level, message string, err error) {
	sc.eventHandler.Handle(events.Event{
		Level:   level,
		Message: message,
		Error:   err,
	})
}

// Debug reports a debug message.
func (sc *StepContext) Debug(message string) {
	sc.event(events.Debug, message, nil)
}

// Debugf reports a debug message with a format string.
func (sc *StepContext) Debugf(format string, args ...any) {
	sc.event(events.Debug, fmt.Sprintf(format, args...), nil)
}

// Info reports an info-level message.
func (sc *StepContext) Info(message string) {
	sc.event(events.Info, message, nil)
}

// Infof reports an info-level message with a format string.
func (sc *StepContext) Infof(format string, args ...any) {
	sc.event(events.Info, fmt.Sprintf(format, args...), nil)
}

// Error reports an error-level message.
func (sc *StepContext) Error(err error, message string) {
	sc.event(events.Error, message, err)
}

// Errorf reports an error-level message with a format string.
func (sc *StepContext) Errorf(err error, format string, args ...any) {
	sc.event(events.Error, fmt.Sprintf(format, args...), err)
}
