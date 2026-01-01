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
				Name:      "init",
				Usage:     "Scaffold a new Shizuka site",
				ArgsUsage: "[directory]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "template", Aliases: []string{"t"}, Value: "minimal", Usage: "Starter template to use"},
					&cli.StringFlag{Name: "name", Aliases: []string{"n"}, Value: "", Usage: "Site name (defaults to directory name)"},
					&cli.BoolFlag{Name: "force", Aliases: []string{"f"}, Value: false, Usage: "Overwrite existing files"},
					&cli.BoolFlag{Name: "list-templates", Aliases: []string{"l"}, Value: false, Usage: "List available templates"},
					&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Value: false, Usage: "Suppress output"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return Init(ctx, cmd)
				},
			},
			{
				Name:  "build",
				Usage: "Build the site into a dist directory",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Value: "shizuka.toml", Usage: "config file path"},
					&cli.StringFlag{Name: "dist", Aliases: []string{"d"}, Value: "", Usage: "output directory (overrides config)"},
					&cli.BoolFlag{Name: "strict", Aliases: []string{"s"}, Value: false, Usage: "fail on warnings (strict mode)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					configPath := cmd.String("config")
					distDir := cmd.String("dist")
					strict := cmd.Bool("strict")
					if strict {
						return BuildStrict(ctx, configPath, distDir)
					}
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
