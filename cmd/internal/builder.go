package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/olimci/shizuka/pkg/build"
)

type Builder struct {
	config *build.Config
}

type BuildResult struct {
	Duration time.Duration
	Error    error
	Reason   string
	Paths    []string
	Number   int
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

func (b *Builder) Build(ctx context.Context) BuildResult {
	start := time.Now()

	steps := []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithMaxWorkers(4),
	}

	err := build.Build(steps, b.config, opts...)
	duration := time.Since(start)

	return BuildResult{
		Duration: duration,
		Error:    err,
	}
}

func (b *Builder) BuildDev(ctx context.Context) BuildResult {
	start := time.Now()

	steps := []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}

	opts := []build.Option{
		build.WithContext(ctx),
		build.WithMaxWorkers(4),
		build.WithDev(), // Enable dev mode for faster rebuilds
	}

	err := build.Build(steps, b.config, opts...)
	duration := time.Since(start)

	return BuildResult{
		Duration: duration,
		Error:    err,
	}
}

func (b *Builder) Config() *build.Config {
	return b.config
}
