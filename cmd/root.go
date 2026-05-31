package cmd

import (
	"context"
	"errors"

	"github.com/olimci/shizuka/internal/logging"
	"github.com/olimci/shizuka/internal/version"
	"github.com/urfave/cli/v3"
)

const banner = "░█▀▀░█░█░▀█▀░▀▀█░█░█░█░█░█▀█\n░▀▀█░█▀█░░█░░▄▀░░█░█░█▀▄░█▀█\n░▀▀▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀░▀░▀░▀"

// defaults
const (
	defaultConfig = "shizuka.jsonc"
	defaultOutput = "dist"
	defaultPort   = 6767
)

func Execute(ctx context.Context, args []string) error {
	app := &cli.Command{
		Name:  "shizuka",
		Usage: "The BEST static site generator",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug mode",
			},
			&cli.StringFlag{
				Name:      "format",
				Value:     "auto",
				Usage:     "Output format: auto, plain, pretty, or json",
				Validator: logging.ValidateFormat,
			},
			&cli.IntFlag{
				Name:    "workers",
				Aliases: []string{"w"},
				Usage:   "Maximum number of concurrent workers",
				Validator: func(workers int) error {
					if workers <= 0 {
						return errors.New("workers must be greater than zero")
					}
					return nil
				},
			},
		},
		Commands: []*cli.Command{
			buildCmd,
			devCmd,
		},
		Version: version.Current().String(),
	}

	return app.Run(ctx, args)
}
