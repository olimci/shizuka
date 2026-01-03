package cmd

import (
	"context"

	"github.com/urfave/cli/v3"
)

func devCmd() *cli.Command {
	return &cli.Command{
		Name:   "dev",
		Usage:  "Start development server with TUI",
		Flags:  devFlags(),
		Action: runDev,
	}
}

func devFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   defaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.StringFlag{
			Name:    "dist",
			Aliases: []string{"d"},
			Usage:   "Output directory (overrides config)",
		},
		&cli.IntFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   defaultDevPort,
			Usage:   "HTTP port",
		},
	}
}

// runDev executes the interactive dev command with TUI
func runDev(ctx context.Context, cmd *cli.Command) error {
	return runDevInteractive(ctx, cmd)
}

// runXDev executes the non-interactive dev command
func runXDev(ctx context.Context, cmd *cli.Command) error {
	return runDevHeadless(ctx, cmd)
}
