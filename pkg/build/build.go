package build

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
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

type dag struct {
	m   map[string]Step
	adj map[string][]string
	deg map[string]int
}

func Build(opt ...options.Option) (err error) {
	opts := options.DefaultOptions().Apply(opt...)

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

	return build(steps, cfg, opts, cfg.Root, configPath)
}

func build(steps []Step, cfg *config.Config, options *options.Options, sourceRoot, configPath string) error {
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
	if err := man.Begin(ctx, cfg, options, buildErrors.Add, ""); err != nil {
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

	readyCh := make(chan string, len(steps))

	state := struct {
		mu      sync.Mutex
		active  int
		queued  int
		remain  int
		stuck   error
		closed  bool
		closeCh func()
	}{
		queued: len(ready),
		remain: len(steps),
	}
	state.closeCh = func() {
		if state.closed {
			return
		}
		close(readyCh)
		state.closed = true
	}

	for _, id := range ready {
		readyCh <- id
	}

	g, gctx := errgroup.WithContext(ctx)
	workerCount := min(maxWorkers, len(steps))
	if workerCount == 0 {
		workerCount = 1
	}

	for range workerCount {
		g.Go(func() error {
			for {
				select {
				case <-gctx.Done():
					return nil
				case id, ok := <-readyCh:
					if !ok {
						return nil
					}

					state.mu.Lock()
					state.active++
					state.queued--
					state.mu.Unlock()

					step := dag.m[id]
					sc := StepContext{
						Manifest:     man,
						Registry:     reg,
						Cache:        cacheReg,
						SourceRoot:   sourceRoot,
						ConfigPath:   configPath,
						ChangedPaths: normalizeChangedPaths(options.ChangedPaths),
						errors:       buildErrors,
					}

					err := step.Fn(gctx, &sc)
					if err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							return nil
						}
						return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
					}

					state.mu.Lock()
					state.active--
					state.remain--
					for _, dep := range dag.adj[id] {
						dag.deg[dep]--
						if dag.deg[dep] == 0 {
							readyCh <- dep
							state.queued++
						}
					}

					switch {
					case state.remain == 0:
						state.closeCh()
					case state.active == 0 && state.queued == 0:
						state.stuck = fmt.Errorf("%w: %v", ErrCircularDependency, stuckStepIDs(dag.deg))
						state.closeCh()
					}
					state.mu.Unlock()
				}
			}
		})
	}

	runErr := g.Wait()
	if runErr == nil && options.Context.Err() != nil {
		runErr = options.Context.Err()
	}
	if runErr == nil {
		state.mu.Lock()
		runErr = state.stuck
		state.mu.Unlock()
	}
	if runErr != nil {
		cancel()
		_ = man.Complete(false)
		return runErr
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
			if err := errMan.Begin(options.Context, cfg, options, nil, ""); err == nil {
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

func stuckStepIDs(deg map[string]int) []string {
	var stuck []string
	for id, d := range deg {
		if d != 0 {
			stuck = append(stuck, id)
		}
	}
	return stuck
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
