package fileutil

import (
	"errors"
	"fmt"
	"os"
)

func EnsureDir(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(root, 0o755); err != nil {
				return fmt.Errorf("directory %q: %w", root, err)
			}
			return nil
		}
		return fmt.Errorf("directory %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", root)
	}
	return nil
}
