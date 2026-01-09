package cmd

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

var Version = version.Current()

func versionCmd() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print the version of shizuka",
		Action: func(ctx context.Context, c *cli.Command) error {
			fmt.Printf("shizuka version %s\n", Version)
			return nil
		},
	}
}
