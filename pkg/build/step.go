package build

import (
	"context"

	"github.com/olimci/shizuka/pkg/manifest"
)

type StepContext struct {
	Ctx context.Context

	Manifest *manifest.Manifest
	Options  *Options

	defers []Step
}

func (ctx *StepContext) Defer(step Step) {
	ctx.defers = append(ctx.defers, step)
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
