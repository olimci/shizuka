package scaffold

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type BuildOptions struct {
	Variables map[string]any
	Force     bool
}

type Template struct {
	Config TemplateCfg
	source *source
	Base   string
}

func (t *Template) Close() error {
	return t.source.Close()
}

// BuildResult contains information about what was created.
type BuildResult struct {
	FilesCreated []string
	DirsCreated  []string
}

// Build scaffolds the template to the target directory.
func (t *Template) Build(targetPath string, opts BuildOptions) (*BuildResult, error) {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("creating target directory: %w", err)
	}

	result := &BuildResult{
		FilesCreated: make([]string, 0),
		DirsCreated:  make([]string, 0),
	}

	err := fs.WalkDir(t.source.fsys, t.Base, func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := relSourcePath(t.Base, src)
		if err != nil {
			return err
		}

		if rel == "." || isScaffoldConfigFile(rel) {
			return nil
		}

		destRelPath := t.transformPath(rel)
		destPath := filepath.Join(targetPath, destRelPath)

		if d.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", destRelPath, err)
			}
			result.DirsCreated = append(result.DirsCreated, destRelPath)
			return nil
		}

		if !opts.Force {
			if _, err := os.Stat(destPath); err == nil {
				return fmt.Errorf("file %s already exists (use force to overwrite)", destRelPath)
			}
		}

		source, err := t.source.fsys.Open(src)
		if err != nil {
			return fmt.Errorf("opening %s: %w", rel, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			_ = source.Close()
			return fmt.Errorf("creating parent directory for %s: %w", destRelPath, err)
		}

		target, err := os.Create(destPath)
		if err != nil {
			_ = source.Close()
			return fmt.Errorf("creating %s: %w", destRelPath, err)
		}

		if matchesGlobs(rel, t.Config.Files.Templates) {
			if err := processTemplate(source, target, opts.Variables); err != nil {
				_ = target.Close()
				_ = source.Close()
				return fmt.Errorf("processing template %s: %w", rel, err)
			}
		} else {
			if _, err := io.Copy(target, source); err != nil {
				_ = target.Close()
				_ = source.Close()
				return fmt.Errorf("copying %s to %s: %w", rel, destRelPath, err)
			}
		}

		if err := target.Close(); err != nil {
			return fmt.Errorf("closing %s: %w", destRelPath, err)
		}
		target = nil

		if err := source.Close(); err != nil {
			return fmt.Errorf("closing %s: %w", rel, err)
		}
		source = nil

		result.FilesCreated = append(result.FilesCreated, destRelPath)
		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

// transformPath applies renames and suffix stripping to get the destination path.
func (t *Template) transformPath(rel string) string {
	dir := path.Dir(rel)
	baseName := path.Base(rel)

	if newName, ok := t.Config.Files.Renames[baseName]; ok {
		baseName = newName
	} else {
		if strings.HasPrefix(baseName, "_") && len(baseName) > 1 {
			baseName = "." + baseName[1:]
		}
	}

	for _, suffix := range t.Config.Files.StripSuffixes {
		if base, ok := strings.CutSuffix(baseName, suffix); ok {
			baseName = base
			break
		}
	}

	if dir == "." {
		return baseName
	}
	return path.Join(dir, baseName)
}

func relSourcePath(base, target string) (string, error) {
	base = path.Clean(base)
	target = path.Clean(target)

	if base == "." {
		return target, nil
	}
	if target == base {
		return ".", nil
	}

	prefix := base + "/"
	if !strings.HasPrefix(target, prefix) {
		return "", fmt.Errorf("%q is not within %q", target, base)
	}

	return strings.TrimPrefix(target, prefix), nil
}
