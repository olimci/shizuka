package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/utils/dag"
	"github.com/olimci/shizuka/pkg/utils/pool"
)

var (
	ErrTaskError   = fmt.Errorf("task error")
	ErrBuildFailed = fmt.Errorf("build failed")
)

type BuildCtx struct {
	StartTime time.Time
	Dev       bool
}

func Build(opt ...options.Option) (err error) {
	opts := options.DefaultOptions().Apply(opt...)
	logger := buildLogger(opts.Logger)

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return err
	}
	logger.Debug("config loaded", "path", opts.ConfigPath, "root", cfg.Root)

	if opts.SiteURL != "" {
		cfg.Site.URL = opts.SiteURL
	}

	graph := dag.New[Step]()
	staticStep := StepStatic(cfg)
	_ = graph.Add(staticStep.ID, staticStep.Deps, staticStep)
	for _, step := range StepContent(cfg, opts) {
		_ = graph.Add(step.ID, step.Deps, step)
	}

	if cfg.Content.Git != nil {
		applyStepPatch(graph, StepGit(cfg))
	}
	if cfg.Artefacts.Headers != nil {
		applyStepPatch(graph, StepHeaders(cfg))
	}
	if cfg.Artefacts.Redirects != nil {
		applyStepPatch(graph, StepRedirects(cfg))
	}
	if cfg.Artefacts.RSS != nil {
		applyStepPatch(graph, StepRSS(cfg))
	}
	if cfg.Artefacts.Sitemap != nil {
		applyStepPatch(graph, StepSitemap(cfg))
	}
	if cfg.Artefacts.Robots != nil {
		applyStepPatch(graph, StepRobots(cfg))
	}
	if cfg.Artefacts.NotFound != nil {
		applyStepPatch(graph, StepNotFound(cfg))
	}
	if cfg.Artefacts.Meta != nil {
		applyStepPatch(graph, StepMeta(cfg))
	}
	logger.Debug("build graph assembled")

	return build(graph, cfg, opts)
}

func applyStepPatch(graph *dag.Graph[Step], patch StepPatch) {
	for _, step := range patch.Steps {
		_ = graph.Add(step.ID, step.Deps, step)
	}
	for _, dep := range patch.dependencies {
		_ = graph.AddDeps(dep.id, []string{dep.dep})
	}
}

func build(graph *dag.Graph[Step], cfg *config.Config, options *options.Options) error {
	startTime := time.Now()
	logger := buildLogger(options.Logger)
	logger.Debug("build started", "source_root", cfg.Root, "config", options.ConfigPath)

	source, err := os.OpenRoot(cfg.Root)
	if err != nil {
		return err
	}
	defer source.Close()

	man := manifest.New()
	reg := registry.New()
	cacheReg := options.CacheRegistry

	if cacheReg != nil {
		registry.Set(cacheReg, ChangedPathsK, options.ChangedPaths)
		defer registry.Delete(cacheReg, ChangedPathsK)
	}

	registry.Set(reg, BuildCtxK, &BuildCtx{
		StartTime: startTime,
		Dev:       options.Dev,
	})

	ctx, cancel := context.WithCancel(options.Context)
	defer cancel()

	var buildErrors = new(errorState)
	if err := man.Start(ctx, cfg, options, buildErrors.Add, ""); err != nil {
		return err
	}

	logger.Debug("manifest started")
	pool := pool.New(ctx, options.MaxWorkers)
	logger.Debug("worker pool started")

	runErr := graph.Run(ctx, options.MaxWorkers, func(ctx context.Context, step Step) error {
		stepStart := time.Now()
		stepLogger := logger.With("step", step.ID)
		stepLogger.Debug("step started")

		regGuard, stepRegistry := reg.Lock(step.RegistryLocks...)
		defer regGuard.Close()

		var stepCache *registry.Scoped
		if cacheReg != nil {
			cacheGuard, scopedCache := cacheReg.Lock(step.CacheLocks...)
			defer cacheGuard.Close()
			stepCache = scopedCache
		}

		sc := StepContext{
			Manifest: man,
			Pool:     pool,
			Registry: stepRegistry,
			Cache:    stepCache,
			Logger:   stepLogger,
			Source:   source,
			errors:   buildErrors,
		}

		err := step.Fn(ctx, &sc)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				stepLogger.Debug("step canceled", "duration", time.Since(stepStart).Truncate(time.Millisecond), "error", err)
				return nil
			}
			stepLogger.Debug("step failed", "duration", time.Since(stepStart).Truncate(time.Millisecond), "error", err)
			return fmt.Errorf("%w (%s): %w", ErrTaskError, step.ID, err)
		}
		stepLogger.Debug("step complete", "duration", time.Since(stepStart).Truncate(time.Millisecond))
		return nil
	})
	if runErr != nil {
		cancel()
		_ = pool.Wait()
		_ = man.Finish(false)
		logger.Debug("build failed", "duration", time.Since(startTime).Truncate(time.Millisecond), "error", runErr)
		return runErr
	}

	workerErr := pool.Wait()
	if workerErr != nil {
		cancel()
		_ = man.Finish(false)
		logger.Debug("build failed", "duration", time.Since(startTime).Truncate(time.Millisecond), "error", workerErr)
		return workerErr
	}

	manifestSuccess := !buildErrors.HasErrors() || options.Dev
	manifestErr := man.Finish(manifestSuccess)
	logger.Debug("manifest complete", "success", manifestSuccess)
	if manifestErr != nil {
		if buildErrors.HasErrors() {
			return &Failure{Errors: buildErrors.Slice()}
		}
		return manifestErr
	}

	if buildErrors.HasErrors() {
		failure := &Failure{Errors: buildErrors.Slice()}
		logger.Debug("build failed", "duration", time.Since(startTime).Truncate(time.Millisecond), "errors", len(failure.Errors))
		return failure
	}

	logger.Debug("build complete", "duration", time.Since(startTime).Truncate(time.Millisecond))
	return nil
}
