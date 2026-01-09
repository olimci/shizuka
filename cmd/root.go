package cmd

import (
	"context"

	"github.com/olimci/prompter"
	"github.com/urfave/cli/v3"
)

const (
	DefaultConfigPath = "shizuka.toml"
	DefaultPort       = 6767
)

var styles = prompter.DefaultStyles()

func Execute(ctx context.Context, args []string) error {
	app := &cli.Command{
		Name:  "shizuka",
		Usage: "A static site generator",
		Commands: []*cli.Command{
			versionCmd(),
			initCmd(),
			buildCmd(),
			devCmd(),

			// noninteractive subcommand group
			xCmd(),
		},
	}

	return app.Run(ctx, args)
}

func xCmd() *cli.Command {
	return &cli.Command{
		Name:  "x",
		Usage: "Non-interactive commands",
		Commands: []*cli.Command{
			versionCmd(),
			xInitCmd(),
			xBuildCmd(),
			xDevCmd(),
		},
	}
}
