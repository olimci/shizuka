package fileutil

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/olimci/shizuka/pkg/utils/set"
)

// WalkFilesFS walks a filesystem tree and returns a set of file paths relative to root.
func WalkFilesFS(fsys fs.FS, root string) (*set.Set[string], error) {
	root = path.Clean(root)
	files := set.New[string]()

	err := fs.WalkDir(fsys, root, func(current string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if current == root {
			return nil
		}

		rel, err := relFSPath(root, current)
		if err != nil {
			return err
		}
		rel = path.Clean(rel)
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

func relFSPath(root, current string) (string, error) {
	root = path.Clean(root)
	current = path.Clean(current)

	if root == "." {
		return current, nil
	}
	if current == root {
		return ".", nil
	}

	prefix := root + "/"
	if !strings.HasPrefix(current, prefix) {
		return "", fmt.Errorf("%q is not within %q", current, root)
	}

	return strings.TrimPrefix(current, prefix), nil
}
