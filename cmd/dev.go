package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/cmd/internal"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/scaffold"
	"github.com/olimci/shizuka/pkg/version"
	"github.com/olimci/shizuka/pkg/watcher"
	"github.com/urfave/cli/v3"
)

func applyDevBuildOptions(opts *config.Options, undev bool) *config.Options {
	if undev {
		return opts
	}

	return opts.
		WithDev().
		WithPageErrorTemplates(map[error]*template.Template{
			build.ErrNoTemplate:       templateFallback.Get(),
			build.ErrTemplateNotFound: templateFallback.Get(),
			nil:                       templateError.Get(),
		}).
		WithErrTemplate(templateBuildError.Get())
}

var devCmd = &cli.Command{
	Name:  "dev",
	Usage: "Start development server with TUI",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   DefaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   DefaultPort,
			Usage:   "HTTP port",
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Value:   0,
			Usage:   "Number of workers to use for building",
		},
		&cli.BoolFlag{
			Name:  "undev",
			Usage: "Run dev server with production-like build options",
		},
	},
	Action: devAction,
}

var xDevCmd = &cli.Command{
	Name:  "dev",
	Usage: "Start development server (non-interactive, logs to stdout)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   DefaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   DefaultPort,
			Usage:   "HTTP port",
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Value:   0,
			Usage:   "Number of workers to use for building",
		},
		&cli.BoolFlag{
			Name:  "undev",
			Usage: "Run dev server with production-like build options",
		},
	},
	Action: xDevAction,
}

func devHeader() string {
	return version.Banner(repoLink) + "\n"
}

func devWatchingStatus(siteURL string) string {
	return fmt.Sprintf("watching for changes, open %s", siteURL)
}

func devAction(ctx context.Context, cmd *cli.Command) error {
	port := fmt.Sprintf(":%d", cmd.Int("port"))
	configPath := cmd.String("config")
	siteURL := fmt.Sprintf("http://localhost:%d/", cmd.Int("port"))

	err := coffee.Do(func(ctx context.Context, c *coffee.Coffee) error {
		defer func() {
			_ = c.Clear()
			_ = c.ClearHeader()
			_ = c.ClearFooter()
		}()

		_ = c.SetWindowTitle("shizuka dev")
		_ = c.LogHeader(devHeader())

		cfg, err := loadDevConfigInteractive(ctx, c, configPath)
		if err != nil {
			return err
		}

		dist, err := os.MkdirTemp("", "shizuka-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dist)

		opts := config.DefaultOptions().
			WithContext(ctx).
			WithConfig(configPath).
			WithOutput(dist).
			WithSiteURL(siteURL)
		opts = applyDevBuildOptions(opts, cmd.Bool("undev"))

		if n := cmd.Int("workers"); n > 0 {
			opts = opts.WithMaxWorkers(n)
		}

		hub := internal.NewReloadHub()

		headersFile := "_headers"
		if cfg.Build.Steps.Headers != nil && cfg.Build.Steps.Headers.Output != "" {
			headersFile = cfg.Build.Steps.Headers.Output
		}
		redirectsFile := "_redirects"
		if cfg.Build.Steps.Redirects != nil && cfg.Build.Steps.Redirects.Output != "" {
			redirectsFile = cfg.Build.Steps.Redirects.Output
		}

		staticHandler := internal.NewStaticHandler(dist, internal.StaticHandlerOptions{
			HeadersFile:   headersFile,
			RedirectsFile: redirectsFile,
			NotFound: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := templateNotFound.Get().Execute(w, nil); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}),
		})

		mux := http.NewServeMux()
		mux.Handle("/_shizuka/reload", http.HandlerFunc(hub.Serve))
		mux.Handle("/", internal.ReloadMiddleware(staticHandler))

		server := &http.Server{
			Addr:    port,
			Handler: mux,
		}

		keysStatus, err := c.Status(devWatchingStatus(siteURL))
		if err != nil {
			return err
		}

		keys, err := c.Keybinds([]coffee.Keybind{
			{Key: "n", Event: "new", Description: "new content"},
			{Key: "r", Event: "rebuild", Description: "rebuild"},
			{Key: "q", Event: "quit", Description: "quit"},
		})
		if err != nil {
			return err
		}
		defer func() {
			_ = keys.Clear()
			_ = keysStatus.Clear()
		}()

		serverErrs := make(chan error, 1)
		go func() {
			serverErrs <- server.ListenAndServe()
		}()

		watch, err := watcher.New(configPath, 200*time.Millisecond)
		if err != nil {
			return err
		}
		defer watch.Close()

		if err := watch.Start(ctx); err != nil {
			return err
		}

		runBuild := func(trigger string) error {
			if err := c.Clear(); err != nil {
				return err
			}

			if err := keysStatus.Working(fmt.Sprintf("building (%s)", trigger)); err != nil {
				return err
			}

			start := time.Now()
			buildErr := build.Build(opts)
			elapsed := time.Since(start).Truncate(time.Millisecond)

			if buildErr != nil {
				_ = keysStatus.Error(fmt.Sprintf("build failed (%s)", elapsed))
				for _, line := range formatBuildError(buildErr) {
					_ = c.Log(line)
				}
				return nil
			}

			_ = c.Logf("built (%s)", elapsed)
			_ = keysStatus.Idle(devWatchingStatus(siteURL))
			hub.Broadcast("reload")
			return nil
		}

		if err := runBuild("initial"); err != nil {
			return err
		}

		for {
			select {
			case <-ctx.Done():
				_ = server.Close()
				return ctx.Err()
			case err := <-serverErrs:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return fmt.Errorf("dev server %q: %w", port, err)
			case ev, ok := <-keys.Events():
				if !ok {
					return nil
				}

				switch ev.Event {
				case "quit":
					_ = server.Close()
					return nil
				case "new":
					res, err := createNewContentInteractive(ctx, c, newRequest{
						ConfigPath: configPath,
					})
					if err != nil {
						_ = c.Logf("new content failed: %s", err)
						_ = keysStatus.Idle(devWatchingStatus(siteURL))
						continue
					}

					_ = c.Logf("created %s", res.Path)
					_ = keysStatus.Idle(devWatchingStatus(siteURL))
				case "rebuild":
					if err := runBuild("manual"); err != nil {
						return err
					}
				}
			case err := <-watch.Errors:
				_ = c.Logf("watch error: %s", err)
			case <-watch.Events:
				if err := runBuild("changes"); err != nil {
					return err
				}
			}
		}
	}, coffee.WithContext(ctx), coffee.WithAltScreen())
	if err != nil && errors.Is(err, coffee.ErrNonInteractive) {
		return xDevAction(ctx, cmd)
	}
	return err
}

func loadDevConfigInteractive(ctx context.Context, c *coffee.Coffee, configPath string) (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err == nil {
		return cfg, nil
	}

	if !(configPath == DefaultConfigPath && errors.Is(err, os.ErrNotExist)) {
		return nil, err
	}

	targetDir, pathErr := filepath.Abs(".")
	if pathErr != nil {
		return nil, pathErr
	}

	confirmed, confirmErr := c.Confirm(fmt.Sprintf("couldn't find %s. create a new site in %s?", configPath, targetDir), false)
	if confirmErr != nil {
		return nil, confirmErr
	}
	if !confirmed {
		return nil, err
	}

	if scaffoldErr := scaffoldSiteForDev(ctx, c); scaffoldErr != nil {
		return nil, scaffoldErr
	}

	return config.Load(configPath)
}

func scaffoldSiteForDev(ctx context.Context, c *coffee.Coffee) error {
	tmpl, coll, err := scaffold.LoadFS(ctx, embed.Scaffold, "scaffold")
	if err != nil {
		return err
	}

	closeFn := func() error { return nil }
	if tmpl != nil {
		closeFn = tmpl.Close
	} else if coll != nil {
		closeFn = coll.Close
	}
	defer closeFn()

	if tmpl == nil {
		if coll == nil {
			return fmt.Errorf("no template found")
		}

		selected, err := c.AwaitSelectDefault("select a template:", coll.Config.Templates.Items, coll.Config.Templates.Default)
		if err != nil {
			return err
		}

		tmpl = coll.Get(selected)
	}

	if tmpl == nil {
		return fmt.Errorf("no template found")
	}

	vars := mergeTemplateVars(tmpl.Config.Variables, nil)

	for key, variable := range tmpl.Config.Variables {
		_ = c.Logf("variable %s (%s): ", variable.Name, variable.Description)
		value, err := c.AwaitInput(
			coffee.WithInputPlaceholder(variable.Description),
			coffee.WithInputValue(fmt.Sprint(vars[key])),
		)
		if err != nil {
			return err
		}

		vars[key] = value
	}

	_ = c.Log("creating site...")

	res, err := tmpl.Build(".", scaffold.BuildOptions{
		Variables: vars,
	})
	if err != nil {
		return err
	}

	_ = c.Log("site created")
	_ = c.Logf("Files: %v", res.FilesCreated)
	_ = c.Logf("Dirs:  %v", res.DirsCreated)
	return nil
}

func xDevAction(ctx context.Context, cmd *cli.Command) error {
	port := fmt.Sprintf(":%d", cmd.Int("port"))
	configPath := cmd.String("config")
	siteURL := fmt.Sprintf("http://localhost:%d/", cmd.Int("port"))

	fmt.Println(devHeader())
	fmt.Println(devWatchingStatus(siteURL))
	fmt.Println()

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	dist, err := os.MkdirTemp("", "shizuka-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dist)

	opts := config.DefaultOptions().
		WithContext(ctx).
		WithConfig(configPath).
		WithOutput(dist).
		WithSiteURL(siteURL)
	opts = applyDevBuildOptions(opts, cmd.Bool("undev"))

	if n := cmd.Int("workers"); n > 0 {
		opts = opts.WithMaxWorkers(n)
	}

	hub := internal.NewReloadHub()

	headersFile := "_headers"
	if cfg.Build.Steps.Headers != nil && cfg.Build.Steps.Headers.Output != "" {
		headersFile = cfg.Build.Steps.Headers.Output
	}
	redirectsFile := "_redirects"
	if cfg.Build.Steps.Redirects != nil && cfg.Build.Steps.Redirects.Output != "" {
		redirectsFile = cfg.Build.Steps.Redirects.Output
	}

	staticHandler := internal.NewStaticHandler(dist, internal.StaticHandlerOptions{
		HeadersFile:   headersFile,
		RedirectsFile: redirectsFile,
		NotFound: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := templateNotFound.Get().Execute(w, nil); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}),
	})

	mux := http.NewServeMux()
	mux.Handle("/_shizuka/reload", http.HandlerFunc(hub.Serve))
	mux.Handle("/", internal.ReloadMiddleware(staticHandler))

	server := &http.Server{
		Addr:    port,
		Handler: mux,
	}

	serverErrs := make(chan error, 1)
	go func() {
		serverErrs <- server.ListenAndServe()
	}()

	watch, err := watcher.New(configPath, 200*time.Millisecond)
	if err != nil {
		return err
	}
	defer watch.Close()

	if err := watch.Start(ctx); err != nil {
		return err
	}

	runBuild := func(trigger string) {
		if buildErr := build.Build(opts); buildErr != nil {
			fmt.Printf("build failed (%s)\n", trigger)
			for _, line := range formatBuildError(buildErr) {
				fmt.Println(line)
			}
			return
		}

		fmt.Printf("built (%s)\n", trigger)
		hub.Broadcast("reload")
	}

	runBuild("initial")

	for {
		select {
		case <-ctx.Done():
			_ = server.Close()
			return ctx.Err()
		case err := <-serverErrs:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("dev server %q: %w", port, err)
		case err := <-watch.Errors:
			fmt.Printf("watch error: %s\n", err)
		case <-watch.Events:
			runBuild("changes")
		}
	}
}
