package cmd

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

const (
	DefaultConfigPath = "shizuka.toml"
	DefaultPort       = 6767
)

const repoLink = "github.com/olimci/shizuka"

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

	if len(args) <= 1 {
		fmt.Println(version.Banner(repoLink) + "\n")
		args = append(args, "help")
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
