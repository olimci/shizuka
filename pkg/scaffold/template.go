package scaffold

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/iofs"
)

type BuildOptions struct {
	Variables map[string]any
	Force     bool
}

type Template struct {
	Config TemplateCfg
	source iofs.Readable
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
func (t *Template) Build(ctx context.Context, targetPath string, opts BuildOptions) (*BuildResult, error) {
	fsy, err := t.source.FS(ctx)
	if err != nil {
		return nil, fmt.Errorf("accessing source: %w", err)
	}

	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return nil, fmt.Errorf("creating target directory: %w", err)
	}

	result := &BuildResult{
		FilesCreated: make([]string, 0),
		DirsCreated:  make([]string, 0),
	}

	err = fs.WalkDir(fsy, t.Base, func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(t.Base, src)
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

		source, err := fsy.Open(src)
		if err != nil {
			return fmt.Errorf("opening %s: %w", rel, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("creating parent directory for %s: %w", destRelPath, err)
		}

		target, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", destRelPath, err)
		}

		if matchesGlobs(rel, t.Config.Files.Templates) {
			if err := processTemplate(source, target, opts.Variables); err != nil {
				return fmt.Errorf("processing template %s: %w", rel, err)
			}
		} else {
			if _, err := io.Copy(target, source); err != nil {
				return fmt.Errorf("copying %s to %s: %w", rel, destRelPath, err)
			}
		}

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
	dir := filepath.Dir(rel)
	baseName := filepath.Base(rel)

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
	return filepath.Join(dir, baseName)
}
