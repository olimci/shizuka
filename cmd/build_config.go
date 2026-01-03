package cmd

import (
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/build"
	"github.com/urfave/cli/v3"
)

func loadBuildConfig(cmd *cli.Command) (string, *build.Config, error) {
	configPath := strings.TrimSpace(cmd.String("config"))
	distOverride := strings.TrimSpace(cmd.String("dist"))

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return "", nil, err
	}

	cfg, err := build.LoadConfig(absConfigPath)
	if err != nil {
		return "", nil, err
	}

	cfgBase := filepath.Dir(absConfigPath)
	resolveBuildPaths(cfg, cfgBase, distOverride)

	return absConfigPath, cfg, nil
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
