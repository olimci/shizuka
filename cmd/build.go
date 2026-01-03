package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/olimci/shizuka/pkg/build"
	"github.com/urfave/cli/v3"
)

func buildCmd() *cli.Command {
	return &cli.Command{
		Name:   "build",
		Usage:  "Build the site (interactive)",
		Flags:  buildFlags(),
		Action: runBuild,
	}
}

func buildFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Value:   defaultConfigPath,
			Usage:   "Config file path",
		},
		&cli.StringFlag{
			Name:    "dist",
			Aliases: []string{"d"},
			Usage:   "Output directory (overrides config)",
		},
		&cli.BoolFlag{
			Name:    "strict",
			Aliases: []string{"s"},
			Usage:   "Fail on warnings (strict mode)",
		},
	}
}

func runBuild(ctx context.Context, cmd *cli.Command) error {
	return runBuildInteractive(ctx, cmd)
}

func runXBuild(ctx context.Context, cmd *cli.Command) error {
	return runBuildWithStyle(ctx, cmd, buildOutputPlain)
}

func runBuildWithStyle(ctx context.Context, cmd *cli.Command, style buildOutputStyle) error {
	configPath, cfg, err := loadBuildConfig(cmd)
	if err != nil {
		return err
	}

	strict := cmd.Bool("strict")

	out := os.Stdout
	printer := newLogPrinter(style, out)

	collector := build.NewDiagnosticCollector(
		build.WithMinLevel(build.LevelDebug),
		build.WithOnReport(func(d build.Diagnostic) {
			if d.Level < build.LevelInfo {
				return
			}
			printer.Print(d)
		}),
	)

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithConfig(configPath),
		build.WithDiagnosticSink(collector),
	}
	if strict {
		opts = append(opts, build.WithFailOnWarn())
	}

	start := time.Now()
	err = build.Build(defaultBuildSteps(), cfg, opts...)
	elapsed := time.Since(start)

	if err != nil {
		if style == buildOutputRich {
			fmt.Fprintf(out, "Build failed (%s)\n", elapsed.Truncate(time.Millisecond))
			// fmt.Fprintf(out, "Logs: %s\n", collector.Summary())
		}
		if style != buildOutputRich && collector.HasLevel(build.LevelInfo) {
			// fmt.Fprintf(out, "logs: %s\n", collector.Summary())
		}
		return err
	}

	if style == buildOutputRich {
		fmt.Fprintf(out, "Build complete (%s)\n", elapsed.Truncate(time.Millisecond))
		// fmt.Fprintf(out, "Logs: %s\n", collector.Summary())
		return nil
	}

	if collector.HasLevel(build.LevelInfo) {
		fmt.Fprintf(out, "logs: %s\n", collector.Summary())
	}
	fmt.Fprintf(out, "built: %s (%s)\n", cfg.Build.OutputDir, elapsed.Truncate(time.Millisecond))

	return nil
}
