package build

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/config"
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

type BuildCtx struct {
	StartTime time.Time

	ConfigPath string
	Dev        bool
}

func Build(opts *config.Options) error {
	configPath, err := config.ResolvePath(filepath.Clean(opts.ConfigPath))
	if err != nil {
		return err
	}
	config, err := config.Load(configPath)
	if err != nil {
		return err
	}
	sourceRoot := config.Root

	if opts.SiteURL != "" {
		config.Site.URL = opts.SiteURL
		if err := config.Validate(); err != nil {
			return err
		}
	}

	steps := make([]Step, 0)
	steps = append(steps, StepStatic())

	useGit := config.Content.Git != nil && config.Content.Git.Enabled
	steps = append(steps, StepContent(useGit)...)

	if config.Headers != nil {
		steps = append(steps, StepHeaders())
	}
	if config.Redirects != nil {
		steps = append(steps, StepRedirects())
	}
	if config.RSS != nil {
		steps = append(steps, StepRSS())
	}
	if config.Sitemap != nil {
		steps = append(steps, StepSitemap())
	}

	return build(steps, config, opts, sourceRoot, configPath)
}

// BuildSteps is a function that builds a site from a DAG of steps.
func build(steps []Step, config *config.Config, options *config.Options, sourceRoot, configPath string) error {
	startTime := time.Now()

	man := manifest.New()
	man.Set(string(OptionsK), options)
	man.Set(string(ConfigK), config)
	man.Set(string(BuildCtxK), &BuildCtx{
		StartTime:  startTime,
		ConfigPath: configPath,
		Dev:        options.Dev,
	})

	buildErrors := &errorState{}

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

	done := 0

	if options.MaxWorkers <= 1 {
		ctx := options.Context
		for len(ready) > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			id := ready[0]
			ready = ready[1:]

			step := dag.m[id]
			sc := StepContext{
				Manifest:   man,
				SourceRoot: sourceRoot,
				errors:     buildErrors,
			}

			if err := step.Fn(ctx, &sc); err != nil {
				return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
			}

			done++
			for _, req := range dag.adj[step.ID] {
				dag.deg[req]--
				if dag.deg[req] == 0 {
					ready = append(ready, req)
				}
			}
		}
	} else {
		g, ctx := errgroup.WithContext(options.Context)
		if options.MaxWorkers > 0 {
			g.SetLimit(options.MaxWorkers)
		}

		var (
			mu       sync.Mutex
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
					Manifest:   man,
					SourceRoot: sourceRoot,
					errors:     buildErrors,
				}

				if err := step.Fn(ctx, &sc); err != nil {
					return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
				}

				var next []string
				mu.Lock()
				done++
				for _, req := range dag.adj[step.ID] {
					dag.deg[req]--
					if dag.deg[req] == 0 {
						next = append(next, req)
					}
				}
				mu.Unlock()

				for _, id := range next {
					schedule(id)
				}

				return nil
			})
		}

		for _, id := range ready {
			schedule(id)
		}

		if err := g.Wait(); err != nil {
			return err
		}
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

	if !buildErrors.HasErrors() {
		if err := man.Build(config, options, buildErrors.Add, ""); err != nil {
			if buildErrors.HasErrors() {
				return &Failure{Errors: buildErrors.Slice()}
			}
			return err
		}
	}

	if buildErrors.HasErrors() {
		failure := &Failure{Errors: buildErrors.Slice()}
		if options.Dev && options.ErrTemplate != nil {
			man := manifest.New()
			man.Emit(manifest.TemplateArtefact(
				manifest.Claim{Owner: "build", Target: "index.html", Canon: "/"},
				options.ErrTemplate,
				failure,
			))
			_ = man.Build(config, options, nil, "")
		}
		return failure
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
