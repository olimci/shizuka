package frontmatter

import (
	"errors"
	"fmt"

	"github.com/olimci/shizuka/pkg/utils/decodeutil"
)

var (
	ErrUnknownType = errors.New("unknown frontmatter type")
	ErrParse       = errors.New("invalid frontmatter")
)

func Extract(doc []byte) (*Frontmatter, []byte, error) {
	return ExtractWithDefaults(doc, "", Defaults{}, nil)
}

func ExtractWithDefaults(doc []byte, defaultSection string, defaults Defaults, bySection map[string]Defaults) (*Frontmatter, []byte, error) {
	b := trimBOM(doc)

	switch fmType, start, end, bodyStart := detect(b); fmType {
	case "yaml":
		return extract(decodeutil.FormatYAML, b[start:end], b[bodyStart:], doc, defaultSection, defaults, bySection)
	case "toml":
		return extract(decodeutil.FormatTOML, b[start:end], b[bodyStart:], doc, defaultSection, defaults, bySection)
	case "json":
		return extract(decodeutil.FormatJSONC, b[start:end], b[bodyStart:], doc, defaultSection, defaults, bySection)
	case "":
		fm := FrontmatterFor(defaultSection, defaults, bySection)
		return &fm, doc, nil
	default:
		return nil, nil, ErrUnknownType
	}
}

func DecodeWithDefaults(format decodeutil.Format, data []byte, defaultSection string, defaults Defaults, bySection map[string]Defaults) (Frontmatter, error) {
	fm, err := BaseForData(format, data, defaultSection, defaults, bySection)
	if err != nil {
		return Frontmatter{}, err
	}
	if err := decodeutil.Unmarshal(format, data, &fm); err != nil {
		return Frontmatter{}, err
	}
	return fm, nil
}

func BaseForData(format decodeutil.Format, data []byte, defaultSection string, defaults Defaults, bySection map[string]Defaults) (Frontmatter, error) {
	section, err := DecodeSection(format, data)
	if err != nil {
		return Frontmatter{}, err
	}
	if section == "" {
		section = defaultSection
	}

	return FrontmatterFor(section, defaults, bySection), nil
}

func DecodeSection(format decodeutil.Format, data []byte) (string, error) {
	var probe struct {
		Section string `toml:"section" yaml:"section" json:"section"`
	}
	if err := decodeutil.Unmarshal(format, data, &probe); err != nil {
		return "", err
	}
	return probe.Section, nil
}

func FrontmatterFor(section string, defaults Defaults, bySection map[string]Defaults) Frontmatter {
	selected := defaults
	if bySection != nil {
		if sectionDefaults, ok := bySection[section]; ok {
			selected = sectionDefaults
		}
	}

	fm := selected.Frontmatter()
	fm.Section = section
	return fm
}

func extract(format decodeutil.Format, data, body, fallback []byte, defaultSection string, defaults Defaults, bySection map[string]Defaults) (*Frontmatter, []byte, error) {
	fm, err := DecodeWithDefaults(format, data, defaultSection, defaults, bySection)
	if err != nil {
		return nil, fallback, fmt.Errorf("%w (%s): %w", ErrParse, format, err)
	}
	return &fm, body, nil
}
