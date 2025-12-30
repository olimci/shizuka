package build

import (
	"context"

	"github.com/olimci/shizuka/pkg/manifest"
)

type StepContext struct {
	Ctx context.Context

	Surface *manifest.Surface
	Options *Options

	defers  []Step
	watches []string
}

type StepCache struct {
	surface *manifest.SurfaceCache
	defers  []Step
	watches []string
}

func (ctx *StepContext) Defer(step Step) {
	ctx.defers = append(ctx.defers, step)
}

func (ctx *StepContext) AddWatch(path string) {
	ctx.watches = append(ctx.watches, path)
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
