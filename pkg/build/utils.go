package build

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/iofs"
	"github.com/olimci/shizuka/pkg/steps"
)

func resolveSource(opts *config.Options) (iofs.Readable, string, error) {
	configPath := opts.ConfigPath
	if opts.Source == nil {
		if filepath.IsAbs(configPath) {
			return iofs.FromOS(filepath.Dir(configPath)), filepath.Base(configPath), nil
		}
		dir := filepath.Dir(configPath)
		if dir != "." {
			return iofs.FromOS(dir), filepath.Base(configPath), nil
		}
		return iofs.FromOS("."), configPath, nil
	}
	if filepath.IsAbs(configPath) {
		return nil, "", fmt.Errorf("config path must be relative when using a custom source: %q", configPath)
	}
	return opts.Source, configPath, nil
}

func openSourceFS(ctx context.Context, source iofs.Readable) (fs.FS, string, error) {
	fsys, err := source.FS(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("accessing source: %w", err)
	}

	root := strings.TrimSpace(source.Root())
	if root == "" {
		root = "."
	}

	if root != "." {
		root, err = steps.CleanFSPath(root)
		if err != nil {
			return nil, "", fmt.Errorf("invalid source root %q: %w", source.Root(), err)
		}
		sub, err := fs.Sub(fsys, root)
		if err != nil {
			return nil, "", fmt.Errorf("source root %q: %w", root, err)
		}
		fsys = sub
		root = "."
	}

	return fsys, root, nil
}
