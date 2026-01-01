package scaffold

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type TemplateManifest struct {
	Name             string            `toml:"name"`
	Description      string            `toml:"description"`
	Version          string            `toml:"version"`
	TemplatePatterns []string          `toml:"template_patterns"`
	Renames          map[string]string `toml:"renames"`
}

type Template struct {
	Manifest TemplateManifest
	FS       fs.FS
	BasePath string
}

type TemplateRegistry struct {
	templates map[string]*Template
}

func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		templates: make(map[string]*Template),
	}
}

func (r *TemplateRegistry) LoadFromFS(fsys fs.FS, rootDir string) error {
	entries, err := fs.ReadDir(fsys, rootDir)
	if err != nil {
		return fmt.Errorf("reading scaffold directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		templatePath := path.Join(rootDir, entry.Name())
		template, err := loadTemplate(fsys, templatePath)
		if err != nil {
			return fmt.Errorf("loading template %q: %w", entry.Name(), err)
		}

		r.templates[template.Manifest.Name] = template
	}

	return nil
}

func (r *TemplateRegistry) Get(name string) (*Template, bool) {
	t, ok := r.templates[name]
	return t, ok
}

func (r *TemplateRegistry) List() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}
func (r *TemplateRegistry) All() []*Template {
	templates := make([]*Template, 0, len(r.templates))
	for _, t := range r.templates {
		templates = append(templates, t)
	}
	return templates
}

func loadTemplate(fsys fs.FS, basePath string) (*Template, error) {
	manifestPath := path.Join(basePath, "template.toml")

	data, err := fs.ReadFile(fsys, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest TemplateManifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if manifest.Renames == nil {
		manifest.Renames = make(map[string]string)
	}

	return &Template{
		Manifest: manifest,
		FS:       fsys,
		BasePath: basePath,
	}, nil
}

func (t *Template) ShouldProcessAsTemplate(filePath string) bool {
	filePath = filepath.ToSlash(filePath)

	for _, pattern := range t.Manifest.TemplatePatterns {
		pattern = filepath.ToSlash(pattern)

		if matched, err := path.Match(pattern, filePath); err == nil && matched {
			return true
		}

		if !strings.Contains(pattern, "/") {
			baseName := path.Base(filePath)
			if matched, err := path.Match(pattern, baseName); err == nil && matched {
				return true
			}
		}
	}
	return false
}

func (t *Template) GetDestinationName(sourceName string) string {
	if dest, ok := t.Manifest.Renames[sourceName]; ok {
		return dest
	}
	return sourceName
}
