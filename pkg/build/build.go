package build

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/profile"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/utils/set"
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

type dag struct {
	m   map[string]Step
	adj map[string][]string
	deg map[string]int
}

func Build(opt ...options.Option) (err error) {
	opts := options.DefaultOptions().Apply(opt...)

	profileState := opts.ProfileState
	if profileState == nil && opts.ProfileOutputPath != "" {
		profileState = profile.NewState()
	}

	defer func() {
		report := profileState.Finalise()
		if opts.ProfileOutputPath == "" {
			return
		}
		if writeErr := profile.WriteJSON(opts.ProfileOutputPath, report); writeErr != nil && err == nil {
			err = writeErr
		}
	}()

	profileState.Begin()

	initSpan := profileState.StartSpan("init", "root", nil)

	configPath, err := config.ResolvePath(filepath.Clean(opts.ConfigPath))
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if opts.SiteURL != "" {
		cfg.Site.URL = opts.SiteURL
		if err := cfg.Validate(); err != nil {
			return err
		}
	}

	steps := make([]Step, 0)
	steps = append(steps, StepStatic(cfg))
	steps = append(steps, StepContent(cfg, opts)...)

	if cfg.Headers != nil {
		steps = append(steps, StepHeaders(cfg))
	}
	if cfg.Redirects != nil {
		steps = append(steps, StepRedirects(cfg))
	}
	if cfg.RSS != nil {
		steps = append(steps, StepRSS(cfg))
	}
	if cfg.Sitemap != nil {
		steps = append(steps, StepSitemap(cfg))
	}

	initSpan.End(nil)

	return build(steps, cfg, opts, cfg.Root, configPath, profileState)
}

func build(steps []Step, cfg *config.Config, options *options.Options, sourceRoot, configPath string, profileState *profile.State) error {
	startTime := time.Now()

	man := manifest.New()
	reg := registry.New()
	cacheReg := options.CacheRegistry
	if cacheReg == nil {
		cacheReg = registry.New()
	}
	registry.SetAs(reg, BuildCtxK, &BuildCtx{
		StartTime:  startTime,
		ConfigPath: configPath,
		Dev:        options.Dev,
	})

	ctx, cancel := context.WithCancel(options.Context)
	defer cancel()

	buildErrors := &errorState{}
	if err := man.Begin(ctx, cfg, options, buildErrors.Add, "", profileState); err != nil {
		return err
	}

	dag, err := newDAG(steps)
	if err != nil {
		_ = man.Complete(false)
		return err
	}

	var ready []string
	for id, d := range dag.deg {
		if d == 0 {
			ready = append(ready, id)
		}
	}
	if len(ready) == 0 {
		_ = man.Complete(false)
		return ErrCircularDependency
	}

	maxWorkers := options.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = len(steps)
		if maxWorkers == 0 {
			maxWorkers = 1
		}
	}

	type result struct {
		id  string
		err error
	}

	results := make(chan result)
	active := 0
	done := 0

	startStep := func(id string) {
		active++

		step := dag.m[id]
		go func() {
			sc := StepContext{
				Manifest:     man,
				Registry:     reg,
				Cache:        cacheReg,
				SourceRoot:   sourceRoot,
				ConfigPath:   configPath,
				ChangedPaths: normalizeChangedPaths(options.ChangedPaths),
				errors:       buildErrors,
			}

			stepSpan := profileState.StartSpan("step", step.ID, nil)
			err := step.Fn(ctx, &sc)
			stepSpan.End(nil)

			if err != nil {
				results <- result{
					id:  step.ID,
					err: fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err),
				}
				return
			}

			results <- result{id: step.ID}
		}()
	}

	for done < len(steps) {
		for active < maxWorkers && len(ready) > 0 {
			id := ready[0]
			ready = ready[1:]
			startStep(id)
		}

		if active == 0 {
			var stuck []string
			for id, d := range dag.deg {
				if d != 0 {
					stuck = append(stuck, id)
				}
			}
			cancel()
			_ = man.Complete(false)
			return fmt.Errorf("%w: %v", ErrCircularDependency, stuck)
		}

		select {
		case <-ctx.Done():
			_ = man.Complete(false)
			return ctx.Err()

		case res := <-results:
			active--
			if res.err != nil {
				cancel()
				_ = man.Complete(false)
				return res.err
			}

			done++
			for _, dep := range dag.adj[res.id] {
				dag.deg[dep]--
				if dag.deg[dep] == 0 {
					ready = append(ready, dep)
				}
			}
		}
	}

	manifestErr := man.Complete(!buildErrors.HasErrors())
	if manifestErr != nil {
		if buildErrors.HasErrors() {
			return &Failure{Errors: buildErrors.Slice()}
		}
		return manifestErr
	}

	if buildErrors.HasErrors() {
		failure := &Failure{Errors: buildErrors.Slice()}
		if options.Dev && options.ErrTemplate != nil {
			errMan := manifest.New()
			if err := errMan.Begin(options.Context, cfg, options, nil, "", profileState); err == nil {
				errMan.Emit(manifest.TemplateArtefact(
					manifest.Claim{Owner: "build", Target: "index.html", Canon: "/"},
					options.ErrTemplate,
					failure,
				))
				_ = errMan.Complete(true)
			}
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
