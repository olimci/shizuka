package build

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/registry"
)

// Step represents the DAG node for a build step
type Step struct {
	ID   string
	Deps []string
	Fn   func(context.Context, *StepContext) error
}

// StepFunc creates a new Step with the given ID, function, and dependencies
func StepFunc(id string, fn func(context.Context, *StepContext) error, deps ...string) Step {
	return Step{
		ID:   id,
		Deps: deps,
		Fn:   fn,
	}
}

// StepContext is the interface for the build step to interact with the build process.
type StepContext struct {
	Manifest     *manifest.Manifest
	Registry     *registry.Registry
	Cache        *registry.Registry
	SourceRoot   string
	ConfigPath   string
	ChangedPaths []string
	errors       *errorState
}

// Error records a source-aware build diagnostic.
func (sc *StepContext) Error(err error, claim manifest.Claim) {
	if err == nil {
		return
	}

	sc.errors.Add(claim, err)
}

func (sc *StepContext) MayHaveChangesUnder(root string) bool {
	if sc.ChangedPaths == nil {
		return true
	}

	root, err := filepath.Abs(root)
	if err != nil {
		root = filepath.Clean(root)
	}
	root = filepath.Clean(root)

	for _, changed := range sc.ChangedPaths {
		changed = filepath.Clean(changed)
		if changed == root || strings.HasPrefix(changed, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (sc *StepContext) ConfigChanged() bool {
	if sc.ChangedPaths == nil || strings.TrimSpace(sc.ConfigPath) == "" {
		return false
	}

	configPath, err := filepath.Abs(sc.ConfigPath)
	if err != nil {
		configPath = filepath.Clean(sc.ConfigPath)
	}
	configPath = filepath.Clean(configPath)

	for _, changed := range sc.ChangedPaths {
		if filepath.Clean(changed) == configPath {
			return true
		}
	}
	return false
}
