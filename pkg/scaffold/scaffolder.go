package scaffold

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Scaffolder is responsible for scaffolding templates into a target directory.
type Scaffolder struct {
	registry *TemplateRegistry
	options  *options
}

func NewScaffolder(registry *TemplateRegistry, opts ...Option) *Scaffolder {
	s := &Scaffolder{
		registry: registry,
		options:  defaultOptions().apply(opts...),
	}

	return s
}

type ScaffoldOptions struct {
	TemplateName string
	TargetDir    string
	Variables    *Variables
	Force        bool
}

type ScaffoldResult struct {
	FilesCreated []string
	DirsCreated  []string
}

func (s *Scaffolder) Scaffold(name, target string, variables *Variables, force bool) (*ScaffoldResult, error) {
	tmpl, ok := s.registry.Get(name)
	if !ok {
		available := s.registry.List()
		return nil, fmt.Errorf("unknown template %q; available templates: %s",
			name, strings.Join(available, ", "))
	}

	if err := s.validateTargetDir(target, force); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return nil, fmt.Errorf("creating target directory: %w", err)
	}

	result := &ScaffoldResult{
		FilesCreated: make([]string, 0),
		DirsCreated:  make([]string, 0),
	}

	templateVars := variables.ToMap()

	err := fs.WalkDir(tmpl.FS, tmpl.BasePath, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(tmpl.BasePath, srcPath)
		if err != nil {
			return err
		}

		if relPath == "." || relPath == "template.toml" {
			return nil
		}

		dir := filepath.Dir(relPath)
		baseName := filepath.Base(relPath)
		destName := tmpl.GetDestinationName(baseName)

		destName = strings.TrimSuffix(destName, ".scaffold")

		var destRelPath string
		if dir == "." {
			destRelPath = destName
		} else {
			destRelPath = filepath.Join(dir, destName)
		}

		destPath := filepath.Join(target, destRelPath)

		if d.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", destRelPath, err)
			}
			result.DirsCreated = append(result.DirsCreated, destRelPath)
			return nil
		}

		content, err := fs.ReadFile(tmpl.FS, srcPath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", relPath, err)
		}

		if tmpl.ShouldProcessAsTemplate(relPath) {
			content, err = s.processTemplate(content, templateVars, relPath)
			if err != nil {
				return fmt.Errorf("processing template %s: %w", relPath, err)
			}
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", destRelPath, err)
		}

		if !force {
			if _, err := os.Stat(destPath); err == nil {
				return fmt.Errorf("file %s already exists (use --force to overwrite)", destRelPath)
			}
		}

		if err := os.WriteFile(destPath, content, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", destRelPath, err)
		}

		result.FilesCreated = append(result.FilesCreated, destRelPath)
		s.logf("  ✓ Created %s\n", destRelPath)

		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

func (s *Scaffolder) validateTargetDir(targetDir string, force bool) error {
	if force {
		return nil
	}

	configPath := filepath.Join(targetDir, "shizuka.toml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("directory already contains shizuka.toml (use --force to overwrite)")
	}

	return nil
}

func (s *Scaffolder) processTemplate(content []byte, vars map[string]any, name string) ([]byte, error) {
	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("executing: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *Scaffolder) logf(format string, args ...any) {
	if !s.options.quiet {
		fmt.Fprintf(s.options.output, format, args...)
	}
}

func (s *Scaffolder) ListTemplates() []TemplateInfo {
	templates := s.registry.All()
	infos := make([]TemplateInfo, 0, len(templates))

	for _, t := range templates {
		infos = append(infos, TemplateInfo{
			Name:        t.Manifest.Name,
			Description: t.Manifest.Description,
			Version:     t.Manifest.Version,
		})
	}

	return infos
}

type TemplateInfo struct {
	Name        string
	Description string
	Version     string
}

type TemplateSource interface {
	Load(registry *TemplateRegistry) error
}

type EmbeddedSource struct {
	FS      fs.FS
	RootDir string
}

func (e *EmbeddedSource) Load(registry *TemplateRegistry) error {
	return registry.LoadFromFS(e.FS, e.RootDir)
}

type DirectorySource struct {
	Path string
}

func (d *DirectorySource) Load(registry *TemplateRegistry) error {
	fsys := os.DirFS(d.Path)
	return registry.LoadFromFS(fsys, ".")
}

type RemoteSource struct {
	URL string
}

func (r *RemoteSource) Load(registry *TemplateRegistry) error {
	// TODO: Implement remote template fetching
	return fmt.Errorf("remote templates not yet implemented")
}

func LoadSources(registry *TemplateRegistry, sources ...TemplateSource) error {
	for _, source := range sources {
		if err := source.Load(registry); err != nil {
			return err
		}
	}
	return nil
}

func NewScaffolderWithEmbedded(fsys fs.FS, rootDir string, opts ...Option) (*Scaffolder, error) {
	registry := NewTemplateRegistry()

	source := &EmbeddedSource{
		FS:      fsys,
		RootDir: rootDir,
	}

	if err := source.Load(registry); err != nil {
		return nil, fmt.Errorf("loading embedded templates: %w", err)
	}

	return NewScaffolder(registry, opts...), nil
}
