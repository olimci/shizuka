package iofs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/utils/fileutils"
)

// FSSource wraps any fs.FS.
func FromFS(fsys fs.FS, root string) *FS {
	return &FS{fs: fsys, root: root}
}

type FS struct {
	fs   fs.FS
	root string
}

func (f *FS) FS(ctx context.Context) (fs.FS, error) {
	return f.fs, nil
}

func (f *FS) Root() string {
	return f.root
}

func (f *FS) Close() error {
	return nil
}

// OSSource wraps any path with an os.DirFS.
func FromOS(path string) *OSFS {
	return &OSFS{path: path}
}

type OSFS struct {
	path string
}

func (o *OSFS) FS(ctx context.Context) (fs.FS, error) {
	return os.DirFS(o.path), nil
}

func (o *OSFS) Root() string {
	return "."
}

func (o *OSFS) Close() error {
	return nil
}

func (o *OSFS) EnsureRoot() error {
	info, err := os.Stat(o.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(o.path, 0o755); err != nil {
				return fmt.Errorf("failed to create build dir %q: %w", o.path, err)
			}
			return nil
		}
		return fmt.Errorf("failed to stat build dir %q: %w", o.path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("build dir %q is not a directory", o.path)
	}
	return nil
}

func (o *OSFS) MkdirAll(rel string, perm fs.FileMode) error {
	return os.MkdirAll(filepath.Join(o.path, rel), perm)
}

func (o *OSFS) Remove(rel string) error {
	return os.Remove(filepath.Join(o.path, rel))
}

func (o *OSFS) RemoveAll(rel string) error {
	return os.RemoveAll(filepath.Join(o.path, rel))
}

func (o *OSFS) Write(rel string, gen WriterFunc, exists bool) error {
	full := filepath.Join(o.path, rel)
	if exists {
		return fileutils.AtomicEdit(full, gen)
	}
	return fileutils.AtomicWrite(full, gen)
}

func (o *OSFS) DisplayPath(rel string) string {
	return filepath.Join(o.path, rel)
}
