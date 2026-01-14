package cmd

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/olimci/prompter"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/events"
	"github.com/urfave/cli/v3"
)

func buildFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

func buildCmd() *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "Build the site (interactive)",
		Flags:  buildFlags(),
		Action: runBuild,
	}
}

func xBuildCmd() *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "Build the site (non-interactive)",
		Flags:  buildFlags(),
		Action: runXBuild,
	}
}

func runBuild(ctx context.Context, cmd *cli.Command) error {
	return prompter.Start(func(ctx context.Context, p *prompter.Prompter) error {
		defer p.Clear()
		opts := config.DefaultOptions().WithContext(ctx)

		if cmd.Bool("dev") {
			opts.
				WithDev().
				WithPageErrorTemplates(map[error]*template.Template{
					build.ErrNoTemplate:       templateFallback.Get(),
					build.ErrTemplateNotFound: templateFallback.Get(),
					nil:                       templateError.Get(),
				}).
				WithErrTemplate(templateBuildError.Get())
		}

		if n := cmd.Int("workers"); n > 0 {
			opts.WithMaxWorkers(n)
		}

		opts.WithEventHandler(events.NewHandlerFunc(func(event events.Event) {
			p.Log(formatEvent(event))
		}))

		status, err := p.Status("building...")
		if err != nil {
			return err
		}

		start := time.Now()
		buildErr, summary := build.Build(opts)
		elapsed := time.Since(start).Truncate(time.Millisecond)

		if buildErr != nil {
			_ = status.Error(fmt.Sprintf("build failed (%s)", elapsed))
			if !hasSummaryEvents(summary) {
				p.Log(buildErr.Error())
			}
		} else {
			_ = status.Success(fmt.Sprintf("built (%s)", elapsed))
		}

		for _, line := range formatSummary(summary) {
			p.Log(line)
		}

		_ = status.Clear()

		if buildErr != nil {
			return buildErr
		}
		return nil
	}, prompter.WithContext(ctx), prompter.WithStyles(styles))
}

func runXBuild(ctx context.Context, cmd *cli.Command) error {
	opts := config.DefaultOptions().WithContext(ctx)

	if cmd.Bool("dev") {
		opts.
			WithDev().
			WithPageErrorTemplates(map[error]*template.Template{
				build.ErrNoTemplate:       templateFallback.Get(),
				build.ErrTemplateNotFound: templateFallback.Get(),
				nil:                       templateError.Get(),
			}).
			WithErrTemplate(templateBuildError.Get())
	}

	if n := cmd.Int("workers"); n > 0 {
		opts.WithMaxWorkers(n)
	}

	opts.WithEventHandler(events.NewHandlerFunc(func(event events.Event) {
		fmt.Println(formatEvent(event))
	}))

	fmt.Println("building...")

	buildErr, summary := build.Build(opts)
	if buildErr != nil {
		fmt.Printf("build failed: %s\n", buildErr)
		for _, line := range formatSummary(summary) {
			fmt.Println(line)
		}
		return buildErr
	}

	fmt.Println("built")
	for _, line := range formatSummary(summary) {
		fmt.Println(line)
	}

	return nil
}
