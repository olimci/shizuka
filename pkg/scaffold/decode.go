package scaffold

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

func decodeConfigFile(filename string, r io.Reader, v any) error {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".toml":
		md, err := toml.NewDecoder(r).Decode(v)
		if err != nil {
			return err
		}
		if undec := md.Undecoded(); len(undec) > 0 {
			return fmt.Errorf("unknown keys in config: %v", undec)
		}
		return nil
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(r)
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
		dec := json.NewDecoder(r)
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
