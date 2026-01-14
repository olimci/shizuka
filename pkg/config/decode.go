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
		md, err := toml.DecodeFile(path, v)
		if err != nil {
			return err
		}
		if undec := md.Undecoded(); len(undec) > 0 {
			return fmt.Errorf("unknown config keys: %v", undec)
		}
		return nil
	case ".yaml", ".yml":
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
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
		dec.DisallowUnknownFields()
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
