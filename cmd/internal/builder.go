package internal

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/build"
)

type Builder struct {
	config           *build.Config
	fallbackTemplate *template.Template
}

type BuildResult struct {
	Duration    time.Duration
	Error       error
	Reason      string
	Paths       []string
	Number      int
	Diagnostics []build.Diagnostic
}

func NewBuilder(configPath string) (*Builder, error) {
	config, err := build.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &Builder{
		config: config,
	}, nil
}

func NewBuilderWithDistOverride(configPath, distDir string) (*Builder, error) {
	config, err := build.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if distDir != "" {
		config.Build.OutputDir = distDir
	}

	return &Builder{
		config: config,
	}, nil
}

// loadFallbackTemplate loads the embedded fallback template
func loadFallbackTemplate() (*template.Template, error) {
	content, err := embed.Templates.ReadFile("templates/fallback.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded fallback template: %w", err)
	}

	tmpl, err := template.New("fallback").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse fallback template: %w", err)
	}

	return tmpl, nil
}

func (b *Builder) Build(ctx context.Context) BuildResult {
	start := time.Now()

	// In prod, only collect warnings and above (skip debug/info noise)
	collector := build.NewDiagnosticCollector(
		build.WithMinLevel(build.LevelWarning),
	)

	steps := []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithMaxWorkers(4),
		build.WithDiagnosticSink(collector),
		build.WithFailOnLevel(build.LevelError), // Fail on errors
	}

	err := build.Build(steps, b.config, opts...)
	duration := time.Since(start)

	return BuildResult{
		Duration:    duration,
		Error:       err,
		Diagnostics: collector.Diagnostics(),
	}
}

func (b *Builder) BuildDev(ctx context.Context) BuildResult {
	start := time.Now()

	// In dev mode, collect everything including debug for verbose output
	collector := build.NewDiagnosticCollector(
		build.WithMinLevel(build.LevelDebug),
	)

	// Load fallback template for dev mode (lazy load and cache)
	if b.fallbackTemplate == nil {
		if tmpl, err := loadFallbackTemplate(); err == nil {
			b.fallbackTemplate = tmpl
		}
		// If loading fails, we just won't use a fallback (non-fatal)
	}

	steps := []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithMaxWorkers(4),
		build.WithDev(),
		build.WithDiagnosticSink(collector),
		build.WithLenientErrors(),               // Demote errors to warnings in dev
		build.WithFailOnLevel(build.LevelError), // Only fail on actual errors (after demotion, there should be none)
	}

	// Add fallback template if available
	if b.fallbackTemplate != nil {
		opts = append(opts, build.WithFallbackTemplate(b.fallbackTemplate))
	}

	err := build.Build(steps, b.config, opts...)
	duration := time.Since(start)

	return BuildResult{
		Duration:    duration,
		Error:       err,
		Diagnostics: collector.Diagnostics(),
	}
}

// BuildStrict is like Build but fails on warnings too
func (b *Builder) BuildStrict(ctx context.Context) BuildResult {
	start := time.Now()

	collector := build.NewDiagnosticCollector(
		build.WithMinLevel(build.LevelWarning),
	)

	steps := []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithMaxWorkers(4),
		build.WithDiagnosticSink(collector),
		build.WithFailOnLevel(build.LevelWarning), // Strict: fail on warnings
	}

	err := build.Build(steps, b.config, opts...)
	duration := time.Since(start)

	return BuildResult{
		Duration:    duration,
		Error:       err,
		Diagnostics: collector.Diagnostics(),
	}
}

func (b *Builder) Config() *build.Config {
	return b.config
}
