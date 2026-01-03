package cmd

import (
	"context"

	"github.com/urfave/cli/v3"
)

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
