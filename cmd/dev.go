package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
	"unicode/utf8"

	"github.com/olimci/shizuka/cmd/internal/console"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/server"
	"github.com/urfave/cli/v3"
)

var devCmd = &cli.Command{
	Name:  "dev",
	Usage: "Start development server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   defaultConfig,
			Usage:   "Config file path",
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   defaultPort,
			Usage:   "Port to listen on",
		},
		&cli.BoolFlag{
			Name:  "undev",
			Usage: "Undev the dev server",
		},
		&cli.BoolFlag{
			Name:  "no-watch",
			Usage: "Disable file watching",
		},
		&cli.BoolFlag{
			Name:  "boring",
			Usage: "Disable fancy terminal output",
		},
	},
	Action: devAction,
}

func devAction(ctx context.Context, cmd *cli.Command) error {
	fancy := !cmd.Bool("boring")

	con, err := console.Open(os.Stdin, os.Stdout, os.Stderr, console.Options{
		HideCursor:     fancy,
		NoEcho:         fancy,
		CleanupSignals: true,
		Context:        ctx,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "console setup failed:", err)
		return handled(err)
	}
	defer con.Close()
	ctx, cancel := context.WithCancel(con.Context())
	defer cancel()

	logger, err := makeLogger(con, cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logger setup failed:", err)
		return handled(err)
	}

	buildOptions := options.Filter(
		options.WithConfigPath(cmd.String("config")),
		options.WithLogger(logger),
		options.If(options.WithMaxWorkers(cmd.Int("workers")), cmd.IsSet("workers")),
		options.If(options.WithDev(true), !cmd.Bool("undev")),
	)

	srv, err := server.New(server.Options{
		Addr:          fmt.Sprintf(":%d", cmd.Int("port")),
		Watch:         !cmd.Bool("no-watch"),
		WatchDebounce: 200 * time.Millisecond,
		Reload:        true,
		Logger:        logger,
		BuildOptions:  buildOptions,
		Build:         build.Build,
	})
	if err != nil {
		logger.Error("dev server setup failed", "error", err)
		return handled(err)
	}
	defer srv.Close()

	if fancy {
		_ = con.ResetView()
	}
	fmt.Fprintln(con.Out, banner)

	go logServerEvents(ctx, con, logger, srv.Events(), fancy)
	go readDevCommands(ctx, cancel, con, srv, logger)

	if err := srv.Start(ctx); err != nil {
		logger.Error("dev server failed", "error", err)
		return handled(err)
	}

	<-ctx.Done()
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		logger.Error("dev server stopped", "error", err)
		return handled(err)
	}
	return nil
}

func readDevCommands(ctx context.Context, cancel context.CancelFunc, con *console.Console, srv *server.Server, logger *slog.Logger) {
	scanner := bufio.NewScanner(con.In)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cmd := scanner.Text()
		if cmd != "" {
			r, _ := utf8.DecodeLastRuneInString(cmd)
			cmd = string(r)
		}
		switch cmd {
		case "q", "Q":
			_ = srv.Close()
			cancel()
			return
		case "r":
			if err := srv.Rebuild(ctx, server.RebuildRequest{Reason: "manual"}); err != nil {
				logger.Error("rebuild failed", "error", err)
			}
		case "R":
			if err := srv.Rebuild(ctx, server.RebuildRequest{Reason: "manual cache reset", ResetCache: true}); err != nil {
				logger.Error("rebuild failed", "error", err)
			}
		case "":
		default:
			logger.Warn("unknown command", "command", cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Error("reading input failed", "error", err)
	}
}

func logServerEvents(ctx context.Context, con *console.Console, logger *slog.Logger, events <-chan server.Event, fancy bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			logServerEvent(con, logger, ev, fancy)
		}
	}
}

func logServerEvent(con *console.Console, logger *slog.Logger, ev server.Event, fancy bool) {
	switch ev.Kind {
	case server.EventBuildStarted:
		if fancy {
			_ = con.ResetView()
			fmt.Fprintln(con.Out, banner)
		}
		logger.Info("building", "reason", ev.Reason)
	case server.EventBuildSucceeded:
		logger.Info("build complete", "reason", ev.Reason, "duration", ev.Duration)
		logger.Info("ready", "url", ev.URL, "commands", "r rebuild, R reset cache, q quit")
	case server.EventBuildFailed:
		logger.Error("build failed", "reason", ev.Reason, "duration", ev.Duration, "error", ev.Err)
		logger.Info("ready", "url", ev.URL, "commands", "r rebuild, R reset cache, q quit")
	case server.EventWatchError:
		logger.Warn("watch error", "error", ev.Err)
	case server.EventServerError:
		logger.Error("server error", "error", ev.Err)
	case server.EventListening:
		logger.Info("listening", "url", ev.URL)
	}
}
