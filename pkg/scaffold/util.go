package scaffold

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/bmatcuk/doublestar/v4"
)

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
	for _, pattern := range patterns {
		if match, err := doublestar.Match(pattern, relPath); err == nil && match {
			return true
		}
	}

	return false
}

func isScaffoldConfigFile(rel string) bool {
	return strings.HasPrefix(rel, TemplateFileBase+".") || strings.HasPrefix(rel, CollectionFileBase+".")
}
