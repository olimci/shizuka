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
	Ctx        context.Context
	Manifest   *manifest.Manifest
	SourceFS   fs.FS
	SourceRoot string
	errors     *errorState
}

// Error records a source-aware build diagnostic.
func (sc *StepContext) Error(err error, claim manifest.Claim) {
	if err == nil {
		return
	}

	sc.errors.Add(claim, err)
}
