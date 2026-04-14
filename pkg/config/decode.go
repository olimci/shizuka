package config

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

func decodeFile(path string, v any) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && ext != ".toml" {
		return fmt.Errorf("unsupported config file type %q (supported: .toml)", ext)
	}

	_, err := toml.DecodeFile(path, v)
	return err
}

func decodeFS(fsys fs.FS, path string, v any) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && ext != ".toml" {
		return fmt.Errorf("unsupported config file type %q (supported: .toml)", ext)
	}

	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}
	if _, err := toml.Decode(string(b), v); err != nil {
		return err
	}
	return nil
}
