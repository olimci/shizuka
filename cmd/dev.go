package cmd

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/olimci/shizuka/cmd/internal"
	"github.com/olimci/shizuka/pkg/build"
	"github.com/urfave/cli/v3"
)

// RunDevServer starts the development server with file watching and auto-rebuild
func RunDevServer(ctx context.Context, cmd *cli.Command) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupts gracefully
	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt)
	defer stopSignals()

	// Load configuration early to derive watch paths and other settings
	configPath := cmd.String("config")
	buildConfig, err := build.LoadConfig(configPath)
	if err != nil {
		return err
	}

	// Derive watch paths from configuration
	watchPaths := []string{
		buildConfig.Build.ContentDir,
		buildConfig.Build.StaticDir,
		configPath, // Watch the config file itself
	}

	// Add template directory from TemplatesGlob
	if templateDir := filepath.Dir(buildConfig.Build.TemplatesGlob); templateDir != "." {
		watchPaths = append(watchPaths, templateDir)
	}

	// Override dist directory if specified via flag
	distDir := cmd.String("dist")
	if distDir == "" {
		distDir = buildConfig.Build.OutputDir
	}

	config := internal.DevServerConfig{
		ConfigPath: configPath,
		DistDir:    distDir,
		Port:       cmd.Int("port"),
		Debounce:   cmd.Duration("debounce"),
		NoUI:       cmd.Bool("no-ui"),
		WatchPaths: watchPaths,
	}

	devServer, err := internal.NewDevServer(config)
	if err != nil {
		return err
	}
	defer devServer.Close()

	return devServer.Run(ctx)
}
