package fileutils

import (
	"io/fs"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/utils/set"
)

// WalkFilesFS walks a filesystem tree and returns a set of file paths relative to root.
func WalkFilesFS(fsys fs.FS, root string) (*set.Set[string], error) {
	root = filepath.Clean(root)
	files := set.New[string]()

	err := fs.WalkDir(fsys, root, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if current == root {
			return nil
		}

		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return nil
		}

		if !d.IsDir() {
			files.Add(rel)
		}
		return nil
	})

	return files, err
}
