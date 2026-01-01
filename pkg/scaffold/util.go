package scaffold

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"text/template"
)

func matchDoubleGlob(pattern, filePath string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		return false
	}

	prefix := strings.TrimSuffix(parts[0], "/")
	suffix := strings.TrimPrefix(parts[1], "/")

	if prefix != "" && !strings.HasPrefix(filePath, prefix+"/") && filePath != prefix {
		return false
	}

	if suffix != "" {
		remaining := filePath
		if prefix != "" {
			remaining = strings.TrimPrefix(filePath, prefix+"/")
		}

		if matched, err := path.Match(suffix, path.Base(remaining)); err == nil && matched {
			return true
		}

		parts := strings.Split(remaining, "/")
		if len(parts) > 0 {
			if matched, err := path.Match(suffix, parts[len(parts)-1]); err == nil && matched {
				return true
			}
		}
	}

	return suffix == ""
}

func processTemplate(source io.Reader, destination io.Writer, vars map[string]any) error {
	content, err := io.ReadAll(source)
	if err != nil {
		return fmt.Errorf("reading: %w", err)
	}

	tmpl, err := template.New("template").Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing: %w", err)
	}

	if err := tmpl.Execute(destination, vars); err != nil {
		return fmt.Errorf("executing: %w", err)
	}

	return nil
}

func matchesGlobs(relPath string, patterns []string) bool {
	relPath = filepath.ToSlash(relPath)

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		if matched, err := path.Match(pattern, relPath); err == nil && matched {
			return true
		}

		if !strings.Contains(pattern, "/") {
			baseName := path.Base(relPath)
			if matched, err := path.Match(pattern, baseName); err == nil && matched {
				return true
			}
		}

		if strings.Contains(pattern, "**") {
			if matchDoubleGlob(pattern, relPath) {
				return true
			}
		}
	}

	return false
}
