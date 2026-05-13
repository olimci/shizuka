package cmd

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/olimci/coffee"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/urfave/cli/v3"
)

var buildCmd = &cli.Command{
	Name:  "build",
	Usage: "Build the site (interactive)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   DefaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.BoolFlag{
			Name:    "dev",
			Aliases: []string{"d"},
			Usage:   "Run in development mode",
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Value:   0,
			Usage:   "Number of workers to use for building",
		},
	},
	Action: buildAction,
}

var xBuildCmd = &cli.Command{
	Name:  "build",
	Usage: "Build the site (non-interactive)",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   DefaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.BoolFlag{
			Name:    "dev",
			Aliases: []string{"d"},
			Usage:   "Run in development mode",
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Value:   0,
			Usage:   "Number of workers to use for building",
		},
	},
	Action: xBuildAction,
}

func buildAction(ctx context.Context, cmd *cli.Command) error {
	err := coffee.Do(func(ctx context.Context, c *coffee.Coffee) error {
		defer func() {
			_ = c.Clear()
		}()

		configPath, err := config.ResolvePath(cmd.String("config"))
		if err != nil {
			return err
		}

		opts := options.Filter(
			options.WithContext(ctx),
			options.WithConfigPath(configPath),
			options.If(options.WithDev(true), cmd.Bool("dev")),
			options.If(options.WithSkipOutputCleanup(true), cmd.Bool("dev")),
			options.If(options.WithPageErrTemplates(map[error]*template.Template{
				build.ErrNoTemplate:       templateFallback.Get(),
				build.ErrTemplateNotFound: templateFallback.Get(),
				nil:                       templateError.Get(),
			}), cmd.Bool("dev")),
			options.If(options.WithErrTemplate(templateBuildError.Get()), cmd.Bool("dev")),
			options.If(options.WithMaxWorkers(cmd.Int("workers")), cmd.Int("workers") > 0),
		)

		status, err := c.Status("building...")
		if err != nil {
			return err
		}

		start := time.Now()
		buildErr := build.Build(opts...)
		elapsed := time.Since(start).Truncate(time.Millisecond)

		if buildErr != nil {
			_ = status.Error(fmt.Sprintf("build failed (%s)", elapsed))
		} else {
			_ = status.Success(fmt.Sprintf("built (%s)", elapsed))
		}

		if err := logBuildError(c, buildErr); err != nil {
			return err
		}

		_ = status.Clear()

		if buildErr != nil {
			return quietError(buildErr)
		}
		return nil
	}, coffee.WithContext(ctx))
	if err != nil && errors.Is(err, coffee.ErrNonInteractive) {
		return xBuildAction(ctx, cmd)
	}
	return err
}

func xBuildAction(ctx context.Context, cmd *cli.Command) error {
	configPath, err := config.ResolvePath(cmd.String("config"))
	if err != nil {
		return err
	}

	opts := options.Filter(
		options.WithContext(ctx),
		options.WithConfigPath(configPath),
		options.If(options.WithDev(true), cmd.Bool("dev")),
		options.If(options.WithSkipOutputCleanup(true), cmd.Bool("dev")),
		options.If(options.WithPageErrTemplates(map[error]*template.Template{
			build.ErrNoTemplate:       templateFallback.Get(),
			build.ErrTemplateNotFound: templateFallback.Get(),
			nil:                       templateError.Get(),
		}), cmd.Bool("dev")),
		options.If(options.WithErrTemplate(templateBuildError.Get()), cmd.Bool("dev")),
		options.If(options.WithMaxWorkers(cmd.Int("workers")), cmd.Int("workers") > 0),
	)

	fmt.Println("building...")

	buildErr := build.Build(opts...)
	if buildErr != nil {
		fmt.Println("build failed")
		for _, line := range formatBuildError(buildErr) {
			fmt.Println(line)
		}
		return quietError(buildErr)
	}

	fmt.Println("built")

	return nil
}
