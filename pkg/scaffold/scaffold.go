package scaffold

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Scaffold struct {
	Config ScaffoldConfig
	source source
	Base   string
}

func (s *Scaffold) Close() error {
	return s.source.Close()
}

// BuildResult contains information about what was created.
type BuildResult struct {
	FilesCreated []string
	DirsCreated  []string
}

// Build scaffolds the template to the target directory.
func (s *Scaffold) Build(ctx context.Context, targetPath string, opts ...Option) (*BuildResult, error) {
	o := defaultOptions().apply(opts...)

	fsy, err := s.source.FS(ctx)
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

	err = fs.WalkDir(fsy, s.Base, func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(s.Base, src)
		if err != nil {
			return err
		}

		if rel == "." || rel == ScaffoldPath {
			return nil
		}

		destRelPath := s.transformPath(rel)
		destPath := filepath.Join(targetPath, destRelPath)

		if d.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", destRelPath, err)
			}
			result.DirsCreated = append(result.DirsCreated, destRelPath)
			return nil
		}

		if !o.force {
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

		if matchesGlobs(rel, s.Config.Files.Templates) {
			if err := processTemplate(source, target, o.variables); err != nil {
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
func (s *Scaffold) transformPath(relPath string) string {
	dir := filepath.Dir(relPath)
	baseName := filepath.Base(relPath)

	if newName, ok := s.Config.Files.Renames[baseName]; ok {
		baseName = newName
	} else {
		if strings.HasPrefix(baseName, "_") && len(baseName) > 1 {
			baseName = "." + baseName[1:]
		}
	}

	for _, suffix := range s.Config.Files.StripSuffixes {
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
