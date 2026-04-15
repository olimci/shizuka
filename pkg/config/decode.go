package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	case slices.Contains(supportedConfigExts, ext):
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

func decodeConfigBytes(path string, b []byte, v any) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "", ".toml":
		_, err := toml.Decode(string(b), v)
		if err != nil {
			return err
		}

		return nil
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(v); err != nil {
			return err
		}
		return nil

	case ".json":
		dec := json.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(v); err != nil {
			return err
		}

		return nil
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
