package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

var Version = version.String()

func Execute(ctx context.Context, args []string) error {
	app := &cli.Command{
		Name:  "shizuka",
		Usage: "A static site generator",
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "print version",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Printf("shizuka version %s\n", Version)
					return nil
				},
			},
			{
				Name:  "build",
				Usage: "Build the site into a dist directory",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "shizuka.toml", Usage: "config file path"},
					&cli.StringFlag{Name: "dist", Aliases: []string{"d"}, Value: "", Usage: "output directory (overrides config)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					configPath := cmd.String("config")
					distDir := cmd.String("dist")
					return Build(ctx, configPath, distDir)
				},
			},
			{
				Name:  "dev",
				Usage: "Start development server with file watching and auto-rebuild",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "shizuka.toml", Usage: "Config file path"},
					&cli.StringFlag{Name: "dist", Aliases: []string{"d"}, Value: "./dist", Usage: "Directory to serve (overrides config)"},
					&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Value: 6767, Usage: "HTTP port"},
					&cli.DurationFlag{Name: "debounce", Value: 250 * time.Millisecond, Usage: "Debounce window for rebuilds"},
					&cli.BoolFlag{Name: "no-ui", Value: false, Usage: "Disable interactive UI and log to stdout only"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return RunDevServer(ctx, cmd)
				},
			},
		},
	}

	return app.Run(ctx, args)
}
