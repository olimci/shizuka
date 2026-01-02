package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/urfave/cli/v3"
)

func runBuild(ctx context.Context, cmd *cli.Command) error {
	return runBuildWithStyle(ctx, cmd, buildOutputRich)
}

func runXBuild(ctx context.Context, cmd *cli.Command) error {
	return runBuildWithStyle(ctx, cmd, buildOutputPlain)
}

type buildOutputStyle int

const (
	buildOutputPlain buildOutputStyle = iota
	buildOutputRich
)

type diagnosticPrinter struct {
	style buildOutputStyle
	out   io.Writer
	mu    sync.Mutex

	levelStyles map[build.DiagnosticLevel]lipgloss.Style
	stepStyle   lipgloss.Style
	sourceStyle lipgloss.Style
}

func newDiagnosticPrinter(style buildOutputStyle, out io.Writer) *diagnosticPrinter {
	p := &diagnosticPrinter{
		style: style,
		out:   out,
	}

	if style != buildOutputRich {
		return p
	}

	colorEnabled := false
	if f, ok := out.(*os.File); ok {
		colorEnabled = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	if !colorEnabled {
		return p
	}

	p.levelStyles = map[build.DiagnosticLevel]lipgloss.Style{
		build.LevelDebug:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")), // muted
		build.LevelInfo:    lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")), // blue
		build.LevelWarning: lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")), // yellow
		build.LevelError:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")), // red
	}
	p.stepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))   // grey
	p.sourceStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")) // text
	return p
}

func (p *diagnosticPrinter) Print(d build.Diagnostic) {
	p.mu.Lock()
	defer p.mu.Unlock()

	line := formatDiagnosticPlain(d)
	if p.style == buildOutputRich && p.levelStyles != nil {
		levelStyle, ok := p.levelStyles[d.Level]
		if ok {
			levelToken := levelStyle.Render(d.Level.String())
			line = formatDiagnosticRich(d, levelToken, p.stepStyle, p.sourceStyle)
		}
	}

	fmt.Fprintln(p.out, line)
}

func formatDiagnosticPlain(d build.Diagnostic) string {
	var b strings.Builder

	b.WriteString(d.Level.String())
	if d.StepID != "" {
		b.WriteString(" [")
		b.WriteString(d.StepID)
		b.WriteString("]")
	}
	b.WriteString(": ")

	if d.Source != "" {
		b.WriteString(d.Source)
		b.WriteString(": ")
	}

	b.WriteString(d.Message)
	if d.Err != nil {
		b.WriteString(": ")
		b.WriteString(d.Err.Error())
	}

	return b.String()
}

func formatDiagnosticRich(d build.Diagnostic, levelToken string, stepStyle, sourceStyle lipgloss.Style) string {
	var b strings.Builder

	b.WriteString(levelToken)
	if d.StepID != "" {
		b.WriteString(" ")
		b.WriteString(stepStyle.Render("[" + d.StepID + "]"))
	}
	b.WriteString(": ")

	if d.Source != "" {
		b.WriteString(sourceStyle.Render(d.Source))
		b.WriteString(": ")
	}

	b.WriteString(d.Message)
	if d.Err != nil {
		b.WriteString(": ")
		b.WriteString(d.Err.Error())
	}

	return b.String()
}

func runBuildWithStyle(ctx context.Context, cmd *cli.Command, style buildOutputStyle) error {
	configPath := strings.TrimSpace(cmd.String("config"))
	distOverride := strings.TrimSpace(cmd.String("dist"))
	strict := cmd.Bool("strict")

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	cfg, err := build.LoadConfig(absConfigPath)
	if err != nil {
		return err
	}

	cfgBase := filepath.Dir(absConfigPath)
	resolveBuildPaths(cfg, cfgBase, distOverride)

	out := os.Stdout
	printer := newDiagnosticPrinter(style, out)

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
		build.WithConfig(absConfigPath),
		build.WithDiagnosticSink(collector),
	}
	if strict {
		opts = append(opts, build.WithFailOnWarn())
	}

	start := time.Now()
	err = build.Build([]build.Step{
		build.StepStatic(),
		build.StepContent(),
	}, cfg, opts...)
	elapsed := time.Since(start)

	if err != nil {
		if style == buildOutputRich {
			fmt.Fprintf(out, "Build failed (%s)\n", elapsed.Truncate(time.Millisecond))
			// fmt.Fprintf(out, "Diagnostics: %s\n", collector.Summary())
		}
		if style != buildOutputRich && collector.HasLevel(build.LevelInfo) {
			// fmt.Fprintf(out, "diagnostics: %s\n", collector.Summary())
		}
		return err
	}

	if style == buildOutputRich {
		fmt.Fprintf(out, "Build complete (%s)\n", elapsed.Truncate(time.Millisecond))
		// fmt.Fprintf(out, "Diagnostics: %s\n", collector.Summary())
		return nil
	}

	if collector.HasLevel(build.LevelInfo) {
		fmt.Fprintf(out, "diagnostics: %s\n", collector.Summary())
	}
	fmt.Fprintf(out, "built: %s (%s)\n", cfg.Build.OutputDir, elapsed.Truncate(time.Millisecond))

	return nil
}

func resolveBuildPaths(cfg *build.Config, baseDir, distOverride string) {
	if distOverride != "" {
		cfg.Build.OutputDir = distOverride
	}

	cfg.Build.OutputDir = resolvePath(baseDir, cfg.Build.OutputDir)
	cfg.Build.TemplatesGlob = resolvePath(baseDir, cfg.Build.TemplatesGlob)
	cfg.Build.StaticDir = resolvePath(baseDir, cfg.Build.StaticDir)
	cfg.Build.ContentDir = resolvePath(baseDir, cfg.Build.ContentDir)
}

func resolvePath(baseDir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}
