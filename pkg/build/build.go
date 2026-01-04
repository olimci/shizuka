package build

import (
	"fmt"
	"sync"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/utils/set"
	"golang.org/x/sync/errgroup"
)

var (
	ErrDuplicateStep        = fmt.Errorf("duplicate step")
	ErrSelfDependency       = fmt.Errorf("self dependency")
	ErrUnresolvedDependency = fmt.Errorf("unresolved dependency")
	ErrCircularDependency   = fmt.Errorf("circular dependency")
	ErrTaskError            = fmt.Errorf("task error")
	ErrBuildFailed          = fmt.Errorf("build failed")
)

func Build(opts ...Option) error {
	o := defaultOptions().Apply(opts...)

	config, err := LoadConfig(o.ConfigPath)
	if err != nil {
		return err
	}

	steps := make([]Step, 0)
	if config.Build.Steps.Static != nil {
		steps = append(steps, StepStatic())
	}
	if config.Build.Steps.Content != nil {
		steps = append(steps, StepContent()...)
	}
	if config.Build.Steps.Headers != nil {
		steps = append(steps, StepHeaders())
	}
	if config.Build.Steps.Redirects != nil {
		steps = append(steps, StepRedirects())
	}

	return buildSteps(steps, config, o)
}

// BuildSteps is a function that builds a site from a DAG of steps.
func buildSteps(steps []Step, config *Config, options *Options) error {
	man := manifest.New()
	man.Set(string(OptionsK), options)
	man.Set(string(ConfigK), config)

	dag, err := newDAG(steps)
	if err != nil {
		return err
	}

	var ready []string
	for id, d := range dag.deg {
		if d == 0 {
			ready = append(ready, id)
		}
	}
	if len(ready) == 0 {
		return ErrCircularDependency
	}

	g, ctx := errgroup.WithContext(options.Context)
	if options.MaxWorkers > 0 {
		g.SetLimit(options.MaxWorkers)
	}

	var (
		mu       sync.Mutex
		done     int
		schedule func(id string)
	)

	schedule = func(id string) {
		step := dag.m[id]
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			sc := StepContext{
				Ctx:      ctx,
				Manifest: man,
				Options:  options,
				StepID:   step.ID,
			}

			if err := step.Fn(&sc); err != nil {
				return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
			}

			var ready []string
			mu.Lock()
			done++
			for _, req := range dag.adj[step.ID] {
				dag.deg[req]--
				if dag.deg[req] == 0 {
					ready = append(ready, req)
				}
			}
			mu.Unlock()

			for _, id := range ready {
				schedule(id)
			}

			return nil
		})
	}

	for _, id := range ready {
		schedule(id)
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("%w: %w", ErrBuildFailed, err)
	}

	if done != len(steps) {
		var stuck []string
		for id, d := range dag.deg {
			if d != 0 {
				stuck = append(stuck, id)
			}
		}
		return fmt.Errorf("%w: %v", ErrCircularDependency, stuck)
	}

	manifestOpts := []manifest.Option{
		manifest.WithBuildDir(config.Build.Output),
		manifest.WithContext(options.Context),
		manifest.WithMaxWorkers(options.MaxWorkers),
	}

	if options.Dev {
		manifestOpts = append(manifestOpts, manifest.IgnoreConflicts())
	}

	if err := man.Build(manifestOpts...); err != nil {
		return fmt.Errorf("%w: %w", ErrBuildFailed, err)
	}

	failLevel := LevelError
	if options.FailOnWarn {
		failLevel = LevelWarning
	}

	if options.DiagnosticSink.HasLevel(failLevel) {
		maxLevel := options.DiagnosticSink.MaxLevel()
		if a := devFailureArtefact(options, DevFailurePageData{
			Summary:     summaryDiagnostics(options.DiagnosticSink),
			FailLevel:   failLevel,
			MaxLevel:    maxLevel,
			Diagnostics: options.DiagnosticSink.DiagnosticsAtLevel(failLevel),
		}); a != nil {
			man.Emit(*a)
			_ = man.Build(manifestOpts...)
		}

		return fmt.Errorf("%w: %s(s) reported during build", ErrBuildFailed, maxLevel)
	}

	return nil
}

// newDAG constructs a DAG from a slice of steps.
func newDAG(steps []Step) (*dag, error) {
	d := &dag{
		m:   make(map[string]Step),
		adj: make(map[string][]string),
		deg: make(map[string]int),
	}

	for _, step := range steps {
		if _, ex := d.m[step.ID]; ex {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateStep, step.ID)
		}
		d.m[step.ID] = step
		d.deg[step.ID] = 0
	}

	for _, step := range steps {
		seen := set.New[string]()
		for _, dep := range step.Deps {
			if step.ID == dep {
				return nil, fmt.Errorf("%w: %s", ErrSelfDependency, step.ID)
			}
			if _, ex := d.m[dep]; !ex {
				return nil, fmt.Errorf("%w: %s", ErrUnresolvedDependency, dep)
			}
			if seen.Has(dep) {
				continue
			}

			seen.Add(dep)
			d.deg[step.ID]++
			d.adj[dep] = append(d.adj[dep], step.ID)
		}
	}

	return d, nil
}

// dag is an internal struct representing a directed acyclic graph
type dag struct {
	m   map[string]Step
	adj map[string][]string
	deg map[string]int
}
