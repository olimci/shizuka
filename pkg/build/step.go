package build

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/events.go"
	"github.com/olimci/shizuka/pkg/manifest"
)

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

// StepContext is the interface for the build step to interact with the build process.
type StepContext struct {
	Ctx          context.Context
	Manifest     *manifest.Manifest
	eventHandler events.Handler
}

func (sc *StepContext) event(level events.Level, message string, err error) {
	sc.eventHandler.Handle(events.Event{
		Level:   level,
		Message: message,
		Error:   err,
	})
}

// Debug reports a debug message
func (sc *StepContext) Debug(message string) {
	sc.event(events.Debug, message, nil)
}

// Debugf reports a debug message with a format string
func (sc *StepContext) Debugf(format string, args ...any) {
	sc.event(events.Debug, fmt.Sprintf(format, args...), nil)
}

// Info reports an info-level message
func (sc *StepContext) Info(message string) {
	sc.event(events.Info, message, nil)
}

// Infof reports an info-level message with a format string
func (sc *StepContext) Infof(format string, args ...any) {
	sc.event(events.Info, fmt.Sprintf(format, args...), nil)
}

// Error reports an error-level message
func (sc *StepContext) Error(err error, message string) {
	sc.event(events.Error, message, err)
}

// Errorf reports an error-level message with a format string
func (sc *StepContext) Errorf(err error, format string, args ...any) {
	sc.event(events.Error, fmt.Sprintf(format, args...), err)
}
