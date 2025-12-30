package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/olimci/shizuka/cmd/internal"
)

// Build performs a single build of the site
func Build(ctx context.Context, configPath, distDir string) error {
	builder, err := internal.NewBuilderWithDistOverride(configPath, distDir)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	result := builder.Build(ctx)

	if result.Error != nil {
		return fmt.Errorf("build failed: %w", result.Error)
	}

	fmt.Printf("OK  built in %s -> %s\n",
		result.Duration.Truncate(time.Millisecond),
		builder.Config().Build.OutputDir)

	return nil
}
