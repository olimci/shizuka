package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/cmd/internal"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/version"
	"github.com/olimci/shizuka/pkg/watcher"
	"github.com/urfave/cli/v3"
)

func devFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

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

func devCmd() *cli.Command {
	flags := append(devFlags(),
		&cli.BoolFlag{
			Name:  "alt-screen",
			Usage: "Use the terminal alt screen for the TUI",
		},
	)

	return &cli.Command{
		Name:   "dev",
		Usage:  "Start development server with TUI",
		Flags:  flags,
		Action: runDev,
	}
}

func xDevCmd() *cli.Command {
	return &cli.Command{
		Name:   "dev",
		Usage:  "Start development server (non-interactive, logs to stdout)",
		Flags:  devFlags(),
		Action: runXDev,
	}
}

func runDev(ctx context.Context, cmd *cli.Command) error {
	port := fmt.Sprintf(":%d", cmd.Int("port"))
	configPath := cmd.String("config")
	siteURL := fmt.Sprintf("http://localhost:%d/", cmd.Int("port"))

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

	coffeeOpts := []coffee.Option{coffee.WithContext(ctx)}
	if cmd.Bool("alt-screen") {
		coffeeOpts = append(coffeeOpts, coffee.WithAltScreen())
	}

	err = coffee.Do(func(ctx context.Context, c *coffee.Coffee) error {
		defer func() {
			_ = c.Clear()
			_ = c.ClearHeader()
			_ = c.ClearFooter()
		}()

		_ = c.SetWindowTitle("shizuka dev")
		_ = c.LogHeader(version.Banner(repoLink))

		keysStatus, err := c.Status("watching for changes")
		if err != nil {
			return err
		}

		keys, err := c.Keybinds([]coffee.Keybind{
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
			_ = keysStatus.Idle("watching for changes")
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
	}, coffeeOpts...)
	if err != nil && errors.Is(err, coffee.ErrNonInteractive) {
		return runXDev(ctx, cmd)
	}
	return err
}

func runXDev(ctx context.Context, cmd *cli.Command) error {
	port := fmt.Sprintf(":%d", cmd.Int("port"))
	configPath := cmd.String("config")
	siteURL := fmt.Sprintf("http://localhost:%d/", cmd.Int("port"))

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
