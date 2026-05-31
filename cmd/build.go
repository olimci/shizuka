package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olimci/shizuka/internal/console"
	"github.com/olimci/shizuka/internal/build"
	"github.com/olimci/shizuka/internal/options"
	"github.com/urfave/cli/v3"
)

var buildCmd = &cli.Command{
	Name:  "build",
	Usage: "Build site",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   defaultConfig,
			Usage:   "Config file path",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   defaultOutput,
			Usage:   "Output directory",
		},
		&cli.BoolFlag{
			Name:  "dev",
			Usage: "Build in dev mode",
		},
	},
	Action: buildAction,
}

func buildAction(ctx context.Context, cmd *cli.Command) error {
	con, err := console.Open(os.Stdin, os.Stdout, os.Stderr, console.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "console setup failed:", err)
		return handled(err)
	}
	defer con.Close()

	logger, err := makeLogger(con, cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logger setup failed:", err)
		return handled(err)
	}

	opts := options.Filter(
		// always
		options.WithContext(ctx),
		options.WithLogger(logger),

		// regular
		options.WithConfigPath(cmd.String("config")),
		options.If(options.WithOutputPath(cmd.String("output")), cmd.IsSet("output")),
		options.If(options.WithMaxWorkers(cmd.Int("workers")), cmd.IsSet("workers")),

		// dev stuff
		options.If(options.WithDev(true), cmd.Bool("dev")),
	)

	logger.Info("building")

	if err := build.Build(opts...); err != nil {
		logger.Error("build failed", "error", err)
		return handled(err)
	}

	logger.Info("build complete")

	return nil
}
