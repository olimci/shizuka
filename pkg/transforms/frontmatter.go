package transforms

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Frontmatter represents the frontmatter of a document
type Frontmatter struct {
	Slug string `toml:"slug" yaml:"slug"`

	Title       string   `toml:"title" yaml:"title"`
	Description string   `toml:"description" yaml:"description"`
	Section     string   `toml:"sections" yaml:"sections"`
	Tags        []string `toml:"tags" yaml:"tags"`

	Date    time.Time `toml:"date" yaml:"date"`
	Updated time.Time `toml:"updated" yaml:"updated"`

	Params     map[string]any `toml:"params" yaml:"params"`
	LiteParams map[string]any `toml:"lite_params" yaml:"lite_params"`

	Template string `toml:"template" yaml:"template"`
	Body     string `toml:"body" yaml:"body"`

	Featured bool `toml:"featured" yaml:"featured"`
	Draft    bool `toml:"draft" yaml:"draft"`
}

var (
	ErrUnknownFrontmatterType = errors.New("unknown frontmatter type")
	ErrNoFrontmatter          = errors.New("no frontmatter")
)

// ExtractFrontmatter extracts the frontmatter from a document
func ExtractFrontmatter(doc []byte) (*Frontmatter, []byte, error) {
	b := trimBOM(doc)

	switch fmType, start, end, bodyStart := detectFrontmatterBlock(b); fmType {
	case "yaml":
		var fm Frontmatter
		if err := yaml.Unmarshal(b[start:end], &fm); err != nil {
			return nil, doc, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}
		return &fm, b[bodyStart:], nil
	case "toml":
		var fm Frontmatter
		if err := toml.Unmarshal(b[start:end], &fm); err != nil {
			return nil, doc, fmt.Errorf("failed to parse TOML frontmatter: %w", err)
		}
		return &fm, b[bodyStart:], nil
	case "":
		return nil, nil, ErrNoFrontmatter
	default:
		return nil, nil, ErrUnknownFrontmatterType
	}
}

// detectFrontmatterBlock detects the frontmatter block in a document, returns (type, start, end, bodyStart)
func detectFrontmatterBlock(b []byte) (string, int, int, int) {
	if len(b) == 0 {
		return "", 0, 0, 0
	}

	switch {
	case hasPrefixAtLineStart(b, []byte("---")):
		return scanFencedBlock(b, []byte("---"), "yaml")
	case hasPrefixAtLineStart(b, []byte("+++")):
		return scanFencedBlock(b, []byte("+++"), "toml")
	default:
		return "", 0, 0, 0
	}
}

// scanFencedBlock scans a fenced block in a document, returns (type, start, end, bodyStart)
func scanFencedBlock(b []byte, fence []byte, kind string) (string, int, int, int) {
	openLineEnd := lineEnd(b, 0)
	line := bytes.TrimRight(b[0:openLineEnd], " \t\r\n")
	if !bytes.Equal(line, fence) {
		return "", 0, 0, 0
	}

	payloadStart := openLineEnd
	i := payloadStart

	for i < len(b) {
		nextEnd := lineEnd(b, i)
		rawLine := b[i:nextEnd]
		lineStripped := bytes.TrimRight(rawLine, " \t\r\n")
		if bytes.Equal(lineStripped, fence) {
			payloadEnd := i
			bodyStart := nextEnd
			return kind, payloadStart, payloadEnd, bodyStart
		}
		i = nextEnd
	}
	return "", 0, 0, 0
}

// hasPrefixAtLineStart detects if a line starts with a prefix
func hasPrefixAtLineStart(b, prefix []byte) bool {
	if !bytes.HasPrefix(b, prefix) {
		return false
	}
	end := lineEnd(b, 0)
	line := bytes.TrimRight(b[:end], " \t\r\n")
	return bytes.Equal(line, prefix)
}

// lineEnd returns the index of the next line end
func lineEnd(b []byte, start int) int {
	i := start
	for i < len(b) && b[i] != '\n' {
		i++
	}
	if i < len(b) && b[i] == '\n' {
		return i + 1
	}
	return i
}

// trimBOM removes the Byte Order Mark (BOM) from the beginning of a byte slice
func trimBOM(b []byte) []byte {
	const (
		b0 = 0xEF
		b1 = 0xBB
		b2 = 0xBF
	)
	if len(b) >= 3 && b[0] == b0 && b[1] == b1 && b[2] == b2 {
		return b[3:]
	}
	return b
}
