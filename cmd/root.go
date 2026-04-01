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
			versionCmd,
			initCmd,
			newCmd,
			buildCmd,
			devCmd,

			// noninteractive subcommand group
			xCmd,
		},
	}

	if len(args) <= 1 {
		fmt.Println(version.Banner(repoLink) + "\n")
		args = append(args, "help")
	}

	return app.Run(ctx, args)
}

var xVersionCmd = &cli.Command{
	Name:  "version",
	Usage: "Print the version of shizuka",
	Action: func(ctx context.Context, c *cli.Command) error {
		fmt.Printf("shizuka version %s\n", Version)
		return nil
	},
}

var xCmd = &cli.Command{
	Name:  "x",
	Usage: "Non-interactive commands",
	Commands: []*cli.Command{
		xVersionCmd,
		xInitCmd,
		xNewCmd,
		xBuildCmd,
		xDevCmd,
	},
}
