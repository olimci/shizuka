package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/olimci/shizuka/cmd/internal"
	"github.com/olimci/shizuka/pkg/build"
)

func Build(ctx context.Context, configPath, distDir string) error {
	builder, err := internal.NewBuilderWithDistOverride(configPath, distDir)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	result := builder.Build(ctx)

	for _, d := range result.Diagnostics {
		prefix := levelPrefix(d.Level)
		if d.Source != "" {
			fmt.Printf("%s %s: %s\n", prefix, d.Source, d.Message)
		} else {
			fmt.Printf("%s %s\n", prefix, d.Message)
		}
	}

	if result.Error != nil {
		return fmt.Errorf("build failed: %w", result.Error)
	}

	summary := summarizeDiagnostics(result.Diagnostics)
	if summary != "" {
		fmt.Printf("OK  built in %s -> %s [%s]\n",
			result.Duration.Truncate(time.Millisecond),
			builder.Config().Build.OutputDir,
			summary)
	} else {
		fmt.Printf("OK  built in %s -> %s\n",
			result.Duration.Truncate(time.Millisecond),
			builder.Config().Build.OutputDir)
	}

	return nil
}

func BuildStrict(ctx context.Context, configPath, distDir string) error {
	builder, err := internal.NewBuilderWithDistOverride(configPath, distDir)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	result := builder.BuildStrict(ctx)

	for _, d := range result.Diagnostics {
		prefix := levelPrefix(d.Level)
		if d.Source != "" {
			fmt.Printf("%s %s: %s\n", prefix, d.Source, d.Message)
		} else {
			fmt.Printf("%s %s\n", prefix, d.Message)
		}
	}

	if result.Error != nil {
		return fmt.Errorf("build failed: %w", result.Error)
	}

	summary := summarizeDiagnostics(result.Diagnostics)
	if summary != "" {
		fmt.Printf("OK  built in %s -> %s [%s]\n",
			result.Duration.Truncate(time.Millisecond),
			builder.Config().Build.OutputDir,
			summary)
	} else {
		fmt.Printf("OK  built in %s -> %s\n",
			result.Duration.Truncate(time.Millisecond),
			builder.Config().Build.OutputDir)
	}

	return nil
}

func levelPrefix(level build.DiagnosticLevel) string {
	switch level {
	case build.LevelDebug:
		return "DBG "
	case build.LevelInfo:
		return "INFO"
	case build.LevelWarning:
		return "WARN"
	case build.LevelError:
		return "ERR "
	default:
		return "    "
	}
}

func summarizeDiagnostics(diagnostics []build.Diagnostic) string {
	counts := make(map[build.DiagnosticLevel]int)
	for _, d := range diagnostics {
		counts[d.Level]++
	}

	if len(counts) == 0 {
		return ""
	}

	var parts []string
	for _, level := range []build.DiagnosticLevel{build.LevelError, build.LevelWarning, build.LevelInfo, build.LevelDebug} {
		if count := counts[level]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, level))
		}
	}
	return joinStrings(parts, ", ")
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
