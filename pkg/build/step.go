package build

import (
	"context"
	"io/fs"

	"github.com/olimci/shizuka/pkg/manifest"
)

// Step represents the DAG node for a build step
type Step struct {
	ID   string
	Deps []string
	Fn   func(context.Context, *StepContext) error
}

// StepFunc creates a new Step with the given ID, function, and dependencies
func StepFunc(id string, fn func(context.Context, *StepContext) error, deps ...string) Step {
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
	ctx        context.Context
	Manifest   *manifest.Manifest
	SourceFS   fs.FS
	SourceRoot string
	errors     *errorState
}

// Context returns the build step context.
func (sc *StepContext) Context() context.Context {
	return sc.ctx
}

// Error records a source-aware build diagnostic.
func (sc *StepContext) Error(err error, claim manifest.Claim) {
	if err == nil {
		return
	}

	sc.errors.Add(claim, err)
}
