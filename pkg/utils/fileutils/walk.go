package fileutils

import (
	"os"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/utils/set"
)

// Walk walks a directory tree and returns a set of files and directories
func Walk(root string) (files *set.Set[string], dirs *set.Set[string], err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	files = set.New[string]()
	dirs = set.New[string]()

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			dirs.Add(rel)
		} else {
			files.Add(rel)
		}

		return nil
	})

	return files, dirs, err
}

// WalkFiles walks a directory tree and returns a set of files
func WalkFiles(root string) (files *set.Set[string], err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	files = set.New[string]()

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}

		if !d.IsDir() {
			files.Add(rel)
		}

		return nil
	})

	return files, err
}

// WalkDirs walks a directory tree and returns a set of directories
func WalkDirs(root string) (dirs *set.Set[string], err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	dirs = set.New[string]()

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			dirs.Add(rel)
		}

		return nil
	})

	return dirs, err
}

// WalkInfo walks a directory tree and returns a map of files and directories with their info
func WalkInfo(root string) (files map[string]os.FileInfo, dirs map[string]os.FileInfo, err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	files = make(map[string]os.FileInfo)
	dirs = make(map[string]os.FileInfo)

	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			dirs[rel] = info
		} else {
			files[rel] = info
		}

		return nil
	})

	return files, dirs, err
}
