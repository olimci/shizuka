package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

var supportedConfigExts = []string{".toml", ".yaml", ".yml", ".json"}

func configCandidates(path string) []string {
	clean := filepath.Clean(path)
	ext := strings.ToLower(filepath.Ext(clean))
	base := strings.TrimSuffix(clean, filepath.Ext(clean))

	switch {
	case ext == "":
		out := make([]string, 0, len(supportedConfigExts))
		for _, candidateExt := range supportedConfigExts {
			out = append(out, base+candidateExt)
		}
		return out
	case isSupportedConfigExt(ext):
		out := make([]string, 0, len(supportedConfigExts))
		out = append(out, clean)
		for _, candidateExt := range supportedConfigExts {
			if candidateExt == ext {
				continue
			}
			out = append(out, base+candidateExt)
		}
		return out
	default:
		return []string{clean}
	}
}

func isSupportedConfigExt(ext string) bool {
	for _, supported := range supportedConfigExts {
		if ext == supported {
			return true
		}
	}
	return false
}

func ResolvePath(path string) (string, error) {
	for _, candidate := range configCandidates(path) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	return filepath.Clean(path), nil
}

func resolvePathFS(fsys fs.FS, path string) (string, error) {
	for _, candidate := range configCandidates(path) {
		if _, err := fs.Stat(fsys, candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	return filepath.Clean(path), nil
}

func decodeConfigBytes(path string, b []byte, v any) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "", ".toml":
		md, err := toml.Decode(string(b), v)
		if err != nil {
			return err
		}
		if undecoded := md.Undecoded(); len(undecoded) > 0 {
			return fmt.Errorf("unknown TOML keys: %v", undecoded)
		}
		return nil
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		if err := dec.Decode(v); err != nil {
			return err
		}
		if err := dec.Decode(&struct{}{}); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		return fmt.Errorf("unexpected extra YAML document")
	case ".json":
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			return err
		}
		if err := dec.Decode(&struct{}{}); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		return fmt.Errorf("unexpected extra content after JSON document")
	default:
		return fmt.Errorf("unsupported config file type %q (supported: .toml, .yaml, .yml, .json)", ext)
	}
}

func decodeFile(path string, v any) error {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return err
	}

	b, err := os.ReadFile(resolvedPath)
	if err != nil {
		return err
	}

	return decodeConfigBytes(resolvedPath, b, v)
}

func decodeFS(fsys fs.FS, path string, v any) error {
	resolvedPath, err := resolvePathFS(fsys, path)
	if err != nil {
		return err
	}

	b, err := fs.ReadFile(fsys, resolvedPath)
	if err != nil {
		return err
	}

	return decodeConfigBytes(resolvedPath, b, v)
}
