package transforms

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// Frontmatter represents the frontmatter of a document
type Frontmatter struct {
	Slug string `toml:"slug" yaml:"slug" json:"slug"`

	Title       string   `toml:"title" yaml:"title" json:"title"`
	Description string   `toml:"description" yaml:"description" json:"description"`
	Section     string   `toml:"sections" yaml:"sections" json:"sections"`
	Tags        []string `toml:"tags" yaml:"tags" json:"tags"`

	Date    time.Time `toml:"date" yaml:"date" json:"date"`
	Updated time.Time `toml:"updated" yaml:"updated" json:"updated"`

	RSS     RSSMeta     `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap SitemapMeta `toml:"sitemap" yaml:"sitemap" json:"sitemap"`

	Params     map[string]any    `toml:"params" yaml:"params" json:"params"`
	LiteParams map[string]any    `toml:"lite_params" yaml:"lite_params" json:"lite_params"`
	Headers    map[string]string `toml:"headers" yaml:"headers" json:"headers"`

	Template string `toml:"template" yaml:"template" json:"template"`
	Body     string `toml:"body" yaml:"body" json:"body"`

	Featured bool `toml:"featured" yaml:"featured" json:"featured"`
	Draft    bool `toml:"draft" yaml:"draft" json:"draft"`
}

type RSSMeta struct {
	Include     bool   `toml:"include" yaml:"include" json:"include"`
	Title       string `toml:"title" yaml:"title" json:"title"`
	Description string `toml:"description" yaml:"description" json:"description"`
	GUID        string `toml:"guid" yaml:"guid" json:"guid"`
}

type SitemapMeta struct {
	Include    bool    `toml:"include" yaml:"include" json:"include"`
	ChangeFreq string  `toml:"changefreq" yaml:"changefreq" json:"changefreq"`
	Priority   float64 `toml:"priority" yaml:"priority" json:"priority"`
}

var (
	ErrUnknownFrontmatterType   = errors.New("unknown frontmatter type")
	ErrFailedToParseFrontmatter = errors.New("failed to parse frontmatter")
	ErrNoFrontmatter            = errors.New("no frontmatter")
)

// ExtractFrontmatter extracts the frontmatter from a document
func ExtractFrontmatter(doc []byte) (*Frontmatter, []byte, error) {
	b := trimBOM(doc)

	switch fmType, start, end, bodyStart := detectFrontmatterBlock(b); fmType {
	case "yaml":
		var fm Frontmatter
		if err := yaml.Unmarshal(b[start:end], &fm); err != nil {
			return nil, doc, fmt.Errorf("%w: %w", ErrFailedToParseFrontmatter, err)
		}
		return &fm, b[bodyStart:], nil
	case "toml":
		var fm Frontmatter
		if err := toml.Unmarshal(b[start:end], &fm); err != nil {
			return nil, doc, fmt.Errorf("%w: %w", ErrFailedToParseFrontmatter, err)
		}
		return &fm, b[bodyStart:], nil
	case "json":
		var fm Frontmatter
		if err := json.Unmarshal(b[start:end], &fm); err != nil {
			return nil, doc, fmt.Errorf("%w: %w", ErrFailedToParseFrontmatter, err)
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
		// JSON frontmatter can be a raw JSON object at the very start of the file.
		if kind, start, end, bodyStart := scanJSONObjectPrefix(b); kind != "" {
			return kind, start, end, bodyStart
		}
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

// scanJSONObjectPrefix scans a JSON object from the start of b, returning (kind, start, end, bodyStart).
func scanJSONObjectPrefix(b []byte) (string, int, int, int) {
	if len(b) == 0 || b[0] != '{' {
		return "", 0, 0, 0
	}

	var (
		depth   = 0
		inStr   = false
		escaped = false
	)

	for i := range b {
		c := b[i]

		if inStr {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inStr = false
			}
			continue
		}

		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end := i + 1
				bodyStart := skipSingleLineEnding(b, end)
				return "json", 0, end, bodyStart
			}
			if depth < 0 {
				return "", 0, 0, 0
			}
		}
	}

	return "", 0, 0, 0
}

func skipSingleLineEnding(b []byte, i int) int {
	if i < len(b) && b[i] == '\r' {
		i++
	}
	if i < len(b) && b[i] == '\n' {
		i++
	}
	return i
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
