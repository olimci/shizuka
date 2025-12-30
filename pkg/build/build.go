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

func Build(steps []Step, config *Config, opts ...Option) (map[string]StepCache, error) {
	o := defaultOptions().Apply(opts...)

	man := manifest.New()
	manifest.Set(man, OptionsK, o)
	manifest.Set(man, ConfigK, config)

	cache := make(map[string]StepCache)

	for len(steps) > 0 {
		dag, err := newDAG(steps)
		if err != nil {
			return nil, err
		}

		var ready []string
		for id, d := range dag.deg {
			if d == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			return nil, ErrCircularDependency
		}

		g, ctx := errgroup.WithContext(o.context)
		if o.maxWorkers > 0 {
			g.SetLimit(o.maxWorkers)
		}

		var (
			mu       sync.Mutex
			done     int
			next     []Step
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

				surface := man.MakeSurface()

				sc := StepContext{
					Ctx:     ctx,
					Surface: surface,
					Options: o,
				}

				if err := step.Func(&sc); err != nil {
					return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
				}

				man.ApplySurface(surface)

				cache[step.ID] = StepCache{
					surface: surface.AsCache(),
					defers:  sc.defers,
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
				next = append(next, sc.defers...)
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
			return nil, fmt.Errorf("%w: %w", ErrBuildFailed, err)
		}

		if done != len(steps) {
			var stuck []string
			for id, d := range dag.deg {
				if d != 0 {
					stuck = append(stuck, id)
				}
			}
			return nil, fmt.Errorf("%w: %v", ErrCircularDependency, stuck)
		}

		steps = next
	}

	manifestOpts := []manifest.Option{
		manifest.WithBuildDir(config.Build.OutputDir),
		manifest.WithContext(o.context),
		manifest.WithMaxWorkers(o.maxWorkers),
	}

	if o.Dev {
		manifestOpts = append(manifestOpts, manifest.IgnoreConflicts())
	}

	if err := man.Build(manifestOpts...); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBuildFailed, err)
	}

	return cache, nil
}

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

type dag struct {
	m   map[string]Step
	adj map[string][]string
	deg map[string]int
}
