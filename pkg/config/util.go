package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

func cleanPatterns(label string, patterns []string) ([]string, error) {
	cleaned := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		normalized := filepath.ToSlash(pattern)
		validationPattern := strings.TrimPrefix(normalized, "/")
		if validationPattern == "" {
			return nil, fmt.Errorf("%s contains an empty pattern", label)
		}
		if !doublestar.ValidatePattern(validationPattern) {
			return nil, fmt.Errorf("%s contains invalid pattern %q", label, pattern)
		}
		cleaned = append(cleaned, normalized)
	}
	return cleaned, nil
}

func (c *Config) resolvePath(label, raw string) (string, error) {
	resolved, err := pathutil.CleanContentPath(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return resolved, nil
}

func (c *Config) resolveGlob(label, raw string) (string, error) {
	resolved, err := pathutil.CleanContentGlob(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return resolved, nil
}

func (c *Config) root() string {
	if c.Root == "" {
		return "."
	}
	return c.Root
}
