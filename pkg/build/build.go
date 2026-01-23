package build

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/events"
	"github.com/olimci/shizuka/pkg/iofs"
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
	StartTime       time.Time
	StartTimestring string

	ConfigPath string
	Dev        bool
}

func Build(opts *config.Options) (error, *events.Summary) {
	return buildWithIO(opts)
}

func BuildWithDestination(opts *config.Options, dest iofs.Writable) (error, *events.Summary) {
	if dest != nil {
		opts = opts.WithDestination(dest)
	}
	return buildWithIO(opts)
}

func buildWithIO(opts *config.Options) (error, *events.Summary) {
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

	configPath, err = cleanFSPath(configPath)
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
	if config.Build.Steps.RSS != nil {
		steps = append(steps, StepRSS())
	}
	if config.Build.Steps.Sitemap != nil {
		steps = append(steps, StepSitemap())
	}

	if opts.OutputPath != "" {

	}

	return build(steps, config, opts, sourceFS, sourceRoot, dest)
}

// BuildSteps is a function that builds a site from a DAG of steps.
func build(steps []Step, config *config.Config, options *config.Options, sourceFS fs.FS, sourceRoot string, out iofs.Writable) (error, *events.Summary) {
	startTime := time.Now()

	man := manifest.New()
	man.Set(string(OptionsK), options)
	man.Set(string(ConfigK), config)
	man.Set(string(BuildCtxK), &BuildCtx{
		StartTime:       startTime,
		StartTimestring: startTime.String(),
		ConfigPath:      options.ConfigPath,
		Dev:             options.Dev,
	})

	collector := events.NewCollector(options.EventHandler)
	summary := func() *events.Summary {
		return collector.Summary()
	}

	dag, err := newDAG(steps)
	if err != nil {
		return err, summary()
	}

	var ready []string
	for id, d := range dag.deg {
		if d == 0 {
			ready = append(ready, id)
		}
	}
	if len(ready) == 0 {
		return ErrCircularDependency, summary()
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
				Ctx:          ctx,
				Manifest:     man,
				SourceFS:     sourceFS,
				SourceRoot:   sourceRoot,
				eventHandler: collector,
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
		return fmt.Errorf("%w: %w", ErrBuildFailed, err), summary()
	}

	if done != len(steps) {
		var stuck []string
		for id, d := range dag.deg {
			if d != 0 {
				stuck = append(stuck, id)
			}
		}
		return fmt.Errorf("%w: %v", ErrCircularDependency, stuck), summary()
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

func resolveSource(opts *config.Options) (iofs.Readable, string, error) {
	configPath := opts.ConfigPath
	if opts.Source == nil {
		if filepath.IsAbs(configPath) {
			return iofs.FromOS(filepath.Dir(configPath)), filepath.Base(configPath), nil
		}
		return iofs.FromOS("."), configPath, nil
	}
	if filepath.IsAbs(configPath) {
		return nil, "", fmt.Errorf("config path must be relative when using a custom source: %q", configPath)
	}
	return opts.Source, configPath, nil
}

func openSourceFS(ctx context.Context, source iofs.Readable) (fs.FS, string, error) {
	fsys, err := source.FS(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("accessing source: %w", err)
	}

	root := strings.TrimSpace(source.Root())
	if root == "" {
		root = "."
	}

	if root != "." {
		root, err = cleanFSPath(root)
		if err != nil {
			return nil, "", fmt.Errorf("invalid source root %q: %w", source.Root(), err)
		}
		sub, err := fs.Sub(fsys, root)
		if err != nil {
			return nil, "", fmt.Errorf("source root %q: %w", root, err)
		}
		fsys = sub
		root = "."
	}

	return fsys, root, nil
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
