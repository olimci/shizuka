package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/olimci/prompter"
	"github.com/olimci/shizuka/cmd/internal"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/events"
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
	}
}

func devCmd() *cli.Command {
	return &cli.Command{
		Name:   "dev",
		Usage:  "Start development server with TUI",
		Flags:  devFlags(),
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
		WithSiteURL(siteURL).
		WithDev().
		WithPageErrorTemplates(map[error]*template.Template{
			build.ErrNoTemplate:       templateFallback.Get(),
			build.ErrTemplateNotFound: templateFallback.Get(),
			nil:                       templateError.Get(),
		}).
		WithErrTemplate(templateBuildError.Get())

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

	return prompter.Start(func(ctx context.Context, p *prompter.Prompter) error {
		opts.WithEventHandler(events.NewHandlerFunc(func(event events.Event) {
			p.Log(formatEvent(event))
		}))

		keysStatus, err := p.StatusKeybinds("watching for changes", []prompter.Keybind{
			{Key: "r", Event: "rebuild", Description: "rebuild"},
			{Key: "q", Event: "quit", Description: "quit"},
		}, prompter.WithKeybindPrompt("keys:"))
		if err != nil {
			return err
		}
		defer keysStatus.Clear()

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
			p.Clear()

			if err := keysStatus.Working(fmt.Sprintf("building (%s)", trigger)); err != nil {
				return err
			}

			start := time.Now()
			buildErr, summary := build.Build(opts)
			elapsed := time.Since(start).Truncate(time.Millisecond)

			if buildErr != nil {
				_ = keysStatus.Error(fmt.Sprintf("build failed (%s)", elapsed))
				p.Logf("build failed (%s)", elapsed)
				if !hasSummaryEvents(summary) {
					p.Log(buildErr.Error())
				}
				for _, line := range formatSummary(summary) {
					p.Log(line)
				}
				return nil
			}

			p.Logf("built (%s)", elapsed)
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
				return err
			case ev := <-keysStatus.Events():
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
				p.Logf("watch error: %s", err)
			case <-watch.Events:
				if err := runBuild("changes"); err != nil {
					return err
				}
			}
		}
	}, prompter.WithContext(ctx), prompter.WithStyles(styles))
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
		WithSiteURL(siteURL).
		WithDev().
		WithPageErrorTemplates(map[error]*template.Template{
			build.ErrNoTemplate:       templateFallback.Get(),
			build.ErrTemplateNotFound: templateFallback.Get(),
			nil:                       templateError.Get(),
		}).
		WithErrTemplate(templateBuildError.Get())

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

	opts.WithEventHandler(events.NewHandlerFunc(func(event events.Event) {
		fmt.Println(formatEvent(event))
	}))

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
		if buildErr, summary := build.Build(opts); buildErr != nil {
			fmt.Printf("build error (%s): %s\n", trigger, buildErr)
			for _, line := range formatSummary(summary) {
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
			return err
		case err := <-watch.Errors:
			fmt.Printf("watch error: %s\n", err)
		case <-watch.Events:
			runBuild("changes")
		}
	}
}
