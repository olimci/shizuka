package cmd

import (
	"context"
	"fmt"

	"github.com/olimci/shizuka/pkg/version"
	"github.com/urfave/cli/v3"
)

var Version = version.String()

func versionCmd() *cli.Command {
	return &cli.Command{
		Name:   "version",
		Usage:  "Print version information",
		Action: runVersion,
	}
}

func runVersion(ctx context.Context, cmd *cli.Command) error {
	fmt.Printf("shizuka version %s\n", Version)
	return nil
}
