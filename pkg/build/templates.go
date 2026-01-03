package build

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

type templateGlobConfig struct {
	rootName string
	funcs    template.FuncMap
}

type TemplateGlobOption func(*templateGlobConfig)

func WithTemplateRootName(name string) TemplateGlobOption {
	return func(cfg *templateGlobConfig) {
		cfg.rootName = name
	}
}

func WithTemplateFuncs(funcs template.FuncMap) TemplateGlobOption {
	return func(cfg *templateGlobConfig) {
		if cfg.funcs == nil {
			cfg.funcs = make(template.FuncMap)
		}
		for k, v := range funcs {
			cfg.funcs[k] = v
		}
	}
}

// parseTemplateGlob parses a glob pattern and returns templates.
// Callers can supply additional options (e.g. funcs) via TemplateGlobOption.
func parseTemplateGlob(pattern string, opts ...TemplateGlobOption) (*template.Template, error) {
	cfg := templateGlobConfig{
		rootName: "site",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no template files found matching pattern: %s", pattern)
	}

	tmpl := template.New(cfg.rootName)
	if cfg.funcs != nil {
		tmpl = tmpl.Funcs(cfg.funcs)
	}

	seenNames := make(map[string]string)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", file, err)
		}

		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

		if existingFile, exists := seenNames[name]; exists {
			return nil, fmt.Errorf("template name conflict: both %s and %s would create template '%s'", existingFile, file, name)
		}
		seenNames[name] = file

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", file, err)
		}
	}

	return tmpl, nil
}
