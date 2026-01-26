package build

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/events"
	"github.com/olimci/shizuka/pkg/iofs"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/steps"
	"github.com/olimci/shizuka/pkg/steps/keys"
	"github.com/olimci/shizuka/pkg/transforms"
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

func Build(opts *config.Options) (error, *events.Summary) {
	source, configPath, err := resolveSource(opts)
	if err != nil {
		return err, nil
	}
	defer source.Close()

	dest := opts.Destination
	if dest != nil {
		defer dest.Close()
	}

	sourceFS, sourceRoot, err := openSourceFS(opts.Context, source)
	if err != nil {
		return err, nil
	}

	configPath, err = steps.CleanFSPath(configPath)
	if err != nil {
		return fmt.Errorf("config path: %w", err), nil
	}

	config, err := config.LoadFS(sourceFS, configPath)
	if err != nil {
		return err, nil
	}

	if strings.TrimSpace(opts.SiteURL) != "" {
		config.Site.URL = strings.TrimSpace(opts.SiteURL)
		if err := config.Validate(); err != nil {
			return err, nil
		}
	}

	buildSteps := make([]steps.Step, 0)
	if config.Build.Steps.Static != nil {
		buildSteps = append(buildSteps, steps.StepStatic())
	}
	if config.Build.Steps.Content != nil {
		buildSteps = append(buildSteps, steps.StepContent()...)
	}
	if config.Build.Steps.Headers != nil {
		buildSteps = append(buildSteps, steps.StepHeaders())
	}
	if config.Build.Steps.Redirects != nil {
		buildSteps = append(buildSteps, steps.StepRedirects())
	}
	if config.Build.Steps.RSS != nil {
		buildSteps = append(buildSteps, steps.StepRSS())
	}
	if config.Build.Steps.Sitemap != nil {
		buildSteps = append(buildSteps, steps.StepSitemap())
	}

	if opts.OutputPath != "" {

	}

	return build(buildSteps, config, opts, sourceFS, sourceRoot, dest)
}

// BuildSteps is a function that builds a site from a DAG of steps.
func build(buildSteps []steps.Step, config *config.Config, options *config.Options, sourceFS fs.FS, sourceRoot string, out iofs.Writable) (error, *events.Summary) {
	startTime := time.Now()

	man := manifest.New()
	man.Set(string(keys.Options), options)
	man.Set(string(keys.Config), config)
	man.Set(string(keys.BuildMeta), &transforms.BuildMeta{
		StartTime:       startTime,
		StartTimestring: startTime.String(),
		ConfigPath:      options.ConfigPath,
		Dev:             options.Dev,
	})

	collector := events.NewCollector(options.EventHandler)
	summary := func() *events.Summary {
		return collector.Summary()
	}

	dag, err := newDAG(buildSteps)
	if err != nil {
		return err, summary()
	}

	runErr := dag.Run(options.Context, options.MaxWorkers, func(ctx context.Context, id string) error {
		step := dag.m[id]
		sc := steps.NewStepContext(ctx, man, sourceFS, sourceRoot, collector)
		if err := step.Fn(&sc); err != nil {
			return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
		}
		return nil
	})
	if runErr != nil {
		if errors.Is(runErr, ErrCircularDependency) {
			return runErr, summary()
		}
		return fmt.Errorf("%w: %w", ErrBuildFailed, runErr), summary()
	}

	if !collector.HasLevel(events.Error) {
		if err := man.Build(config, options, collector, out); err != nil {
			return fmt.Errorf("%w: %w", ErrBuildFailed, err), summary()
		}
	}

	if collector.HasLevel(events.Error) {
		if options.Dev && options.ErrTemplate != nil {
			man := manifest.New()
			man.Emit(manifest.TemplateArtefact(
				manifest.Claim{Owner: "build", Target: "index.html", Canon: "/"},
				options.ErrTemplate,
				collector.Summary(),
			))
			_ = man.Build(config, options, new(events.NoopHandler), out)
		}
		summary := summary()
		return fmt.Errorf("%w: %v", ErrBuildFailed, summary), summary
	}

	return nil, summary()
}

// newDAG constructs a DAG from a slice of steps.
func newDAG(buildSteps []steps.Step) (*dag, error) {
	d := &dag{
		m:   make(map[string]steps.Step),
		adj: make(map[string][]string),
		deg: make(map[string]int),
	}

	for _, step := range buildSteps {
		if _, ex := d.m[step.ID]; ex {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateStep, step.ID)
		}
		d.m[step.ID] = step
		d.deg[step.ID] = 0
	}

	for _, step := range buildSteps {
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
	m   map[string]steps.Step
	adj map[string][]string
	deg map[string]int
}

func (d *dag) Run(ctx context.Context, maxWorkers int, run func(ctx context.Context, id string) error) error {
	deg := make(map[string]int, len(d.deg))
	maps.Copy(deg, d.deg)

	var ready []string
	for id, count := range deg {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	if len(ready) == 0 {
		return ErrCircularDependency
	}

	g, ctx := errgroup.WithContext(ctx)
	if maxWorkers > 0 {
		g.SetLimit(maxWorkers)
	}

	var (
		mu       sync.Mutex
		done     int
		schedule func(id string)
	)

	schedule = func(id string) {
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := run(ctx, id); err != nil {
				return err
			}

			var ready []string
			mu.Lock()
			done++
			for _, req := range d.adj[id] {
				deg[req]--
				if deg[req] == 0 {
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
		return err
	}

	if done != len(d.m) {
		var stuck []string
		for id, count := range deg {
			if count != 0 {
				stuck = append(stuck, id)
			}
		}
		return fmt.Errorf("%w: %v", ErrCircularDependency, stuck)
	}

	return nil
}
