package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

func decodeFile(path string, v any) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case "", ".toml":
		_, err := toml.DecodeFile(path, v)
		return err
	case ".yaml", ".yml":
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		dec := yaml.NewDecoder(f)
		if err := dec.Decode(v); err != nil {
			return err
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			if err == nil {
				return fmt.Errorf("unexpected extra YAML document")
			}
			return err
		}
		return nil
	case ".json":
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		dec := json.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(v); err != nil {
			return err
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			if err == nil {
				return fmt.Errorf("unexpected extra content after JSON document")
			}
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported config file type %q (supported: .toml, .yaml, .yml, .json)", ext)
	}
}
