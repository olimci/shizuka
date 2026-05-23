package build

import (
	"context"
	"log/slog"
	"os"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/utils/pool"
)

// Step represents the DAG node for a build step.
type Step struct {
	ID            string
	Deps          []string
	RegistryLocks []registry.Lock
	CacheLocks    []registry.Lock
	Fn            func(context.Context, *StepContext) error
}

// StepFunc creates a new Step with the given ID, function, and dependencies
func StepFunc(id string, fn func(context.Context, *StepContext) error, deps ...string) Step {
	return Step{
		ID:   id,
		Deps: deps,
		Fn:   fn,
	}
}

func (s Step) Registry(locks ...registry.Lock) Step {
	s.RegistryLocks = append(s.RegistryLocks, locks...)
	return s
}

func (s Step) Cache(locks ...registry.Lock) Step {
	s.CacheLocks = append(s.CacheLocks, locks...)
	return s
}

// StepPatch describes an optional build graph contribution.
type StepPatch struct {
	Steps        []Step
	dependencies []stepDependency
}

type stepDependency struct {
	id  string
	dep string
}

// StepPatchFunc creates a patch that contributes one or more optional steps.
func StepPatchFunc(steps ...Step) StepPatch {
	return StepPatch{
		Steps: steps,
	}
}

// AddDependency declares that an existing graph step must run after dependency
// when the patch is applied.
func (p StepPatch) AddDependency(id, dependency string) StepPatch {
	p.dependencies = append(p.dependencies, stepDependency{id: id, dep: dependency})
	return p
}

// StepContext is the interface for the build step to interact with the build process.
type StepContext struct {
	Manifest *manifest.Manifest
	Pool     *pool.Pool
	Registry *registry.Scoped
	Cache    *registry.Scoped
	Logger   *slog.Logger

	Source *os.Root

	// unexported to steps
	errors *errorState
}

// Error records an error
func (sc *StepContext) Error(err error, claim manifest.Claim) {
	if err == nil {
		return
	}

	if sc.errors != nil {
		sc.errors.Add(claim, err)
	}
	if sc.Logger != nil {
		sc.Logger.Warn("build error",
			"error", err,
			"claim_owner", claim.Owner,
			"claim_source", claim.Source,
			"claim_target", claim.Target,
		)
	}
}
