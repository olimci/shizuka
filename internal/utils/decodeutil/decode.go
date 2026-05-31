package decodeutil

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/olimci/roundtrip/json"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTOML  Format = "toml"
	FormatYAML  Format = "yaml"
	FormatJSON  Format = "json"
	FormatJSONC Format = "jsonc"
)

func FormatExt(ext string) (Format, bool) {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "toml", ".toml":
		return FormatTOML, true
	case "yaml", "yml", ".yaml", ".yml":
		return FormatYAML, true
	case "json", ".json":
		return FormatJSON, true
	case "jsonc", ".jsonc":
		return FormatJSONC, true
	default:
		return "", false
	}
}

func UnmarshalExt(ext string, data []byte, v any) error {
	format, ok := FormatExt(ext)
	if !ok {
		return fmt.Errorf("unsupported decode format for extension %q", ext)
	}
	return Unmarshal(format, data, v)
}

func Unmarshal(format Format, data []byte, v any) error {
	return Decode(format, bytes.NewReader(data), v)
}

func DecodeExt(ext string, r io.Reader, v any) error {
	format, ok := FormatExt(ext)
	if !ok {
		return fmt.Errorf("unsupported decode format for extension %q", ext)
	}
	return Decode(format, r, v)
}

func Decode(format Format, r io.Reader, v any) error {
	switch format {
	case FormatTOML:
		_, err := toml.NewDecoder(r).Decode(v)
		return err
	case FormatYAML:
		return yaml.NewDecoder(r).Decode(v)
	case FormatJSON:
		_, err := json.NewDecoder(r).Decode(v)
		return err
	case FormatJSONC:
		_, err := json.NewJSONCDecoder(r).Decode(v)
		return err
	default:
		return fmt.Errorf("unsupported decode format %q", format)
	}
}
