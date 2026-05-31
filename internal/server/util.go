package server

import (
	"io"
	"log/slog"
	"path/filepath"

	"github.com/olimci/shizuka/internal/config"
)

func headersFile(cfg *config.Config) string {
	if cfg != nil && cfg.Artefacts.Headers != nil {
		return cfg.Artefacts.Headers.Path
	}
	return ""
}

func redirectsFile(cfg *config.Config) string {
	if cfg != nil && cfg.Artefacts.Redirects != nil {
		return cfg.Artefacts.Redirects.Path
	}
	return ""
}

func shouldRefreshConfig(configPath string, changedPaths []string) bool {
	if changedPaths == nil {
		return true
	}
	configPath = filepath.Clean(configPath)
	configAbs, err := filepath.Abs(configPath)
	if err == nil {
		configPath = filepath.Clean(configAbs)
	}
	for _, changed := range changedPaths {
		changed = filepath.Clean(changed)
		changedAbs, err := filepath.Abs(changed)
		if err == nil {
			changed = filepath.Clean(changedAbs)
		}
		if changed == configPath {
			return true
		}
	}
	return false
}

func serverLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return logger.With("component", "server")
}

func lazySend[T any](ch chan<- T, value T) {
	select {
	case ch <- value:
	default:
	}
}
