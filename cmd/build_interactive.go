package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/olimci/prompter"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/urfave/cli/v3"
)

func runBuildInteractive(ctx context.Context, cmd *cli.Command) error {
	configPath, cfg, err := loadBuildConfig(cmd)
	if err != nil {
		return err
	}

	strict := cmd.Bool("strict")

	return prompter.Start(func(p *prompter.Prompter) error {
		status, err := p.Status("Building")
		if err != nil {
			return err
		}
		if err := status.Working("Preparing build"); err != nil {
			return err
		}

		collector := build.NewDiagnosticCollector(
			build.WithMinLevel(build.LevelDebug),
			build.WithOnReport(func(d build.Diagnostic) {
				if d.Level < build.LevelInfo {
					return
				}
				line := formatLogPlain(d)
				_ = status.Message(line)
				p.Log(line)
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
			_ = status.Message("Build failed")
			_ = status.Clear()
			p.Log(fmt.Sprintf("Build failed (%s)", elapsed.Truncate(time.Millisecond)))
			if collector.HasLevel(build.LevelInfo) {
				p.Log(fmt.Sprintf("logs: %s", collector.Summary()))
			}
			return err
		}

		_ = status.Message("Build complete")
		_ = status.Clear()
		if collector.HasLevel(build.LevelInfo) {
			p.Log(fmt.Sprintf("logs: %s", collector.Summary()))
		}
		p.Log(fmt.Sprintf("built: %s (%s)", cfg.Build.OutputDir, elapsed.Truncate(time.Millisecond)))

		return nil
	}, prompter.WithContext(ctx))
}
