package transforms

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	gm "github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"
)

var (
	ErrFailedToParsePage      = errors.New("failed to parse page")
	ErrUnsupportedContentType = errors.New("unsupported content type")
)

// Page represents a page in the site
type Page struct {
	Meta PageMeta
	Tree *PageNode

	Slug  string
	Canon string

	Title       string
	Description string
	Section     string
	Tags        []string

	RSS     RSSMeta
	Sitemap SitemapMeta

	Date    time.Time
	Updated time.Time
	PubDate time.Time

	Params     map[string]any
	LiteParams map[string]any
	Headers    map[string]string

	Body template.HTML

	Featured bool
	Draft    bool
}

// Lite returns a lite representation of the page
func (p *Page) Lite() *PageLite {
	return &PageLite{
		Slug:        p.Slug,
		Canon:       p.Canon,
		Title:       p.Title,
		Description: p.Description,
		Section:     p.Section,
		Tags:        p.Tags,
		Date:        p.Date,
		Updated:     p.Updated,
		PubDate:     p.PubDate,
		LiteParams:  p.LiteParams,
		Featured:    p.Featured,
		Draft:       p.Draft,
	}
}

// PageLite is a lite representation of a page, used for links etc
type PageLite struct {
	Slug  string
	Canon string

	Title       string
	Description string
	Section     string
	Tags        []string

	Date    time.Time
	Updated time.Time
	PubDate time.Time

	LiteParams map[string]any

	Featured bool
	Draft    bool
}

// PageMeta represents metadata for a page
type PageMeta struct {
	Source  string
	URLPath string
	Target  string

	Template string

	BuildTime       time.Time
	BuildTimeString string
}

// PageTemplate is the struct from which page templates are built
type PageTemplate struct {
	Page  Page
	Site  Site
	Error error
}

// BuildPage builds a page from a file
func BuildPage(source string, md gm.Markdown) (*Page, error) {
	var (
		fm   *Frontmatter
		body string
		err  error
	)

	switch ext := filepath.Ext(filepath.Base(source)); ext {
	case ".md":
		fm, body, err = buildMD(source, md)
	case ".toml":
		fm, body, err = buildTOML(source)
	case ".yaml", ".yml":
		fm, body, err = buildYaml(source)
	case ".json":
		fm, body, err = buildJSON(source)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	if err != nil {
		return nil, err
	}

	return &Page{
		Meta: PageMeta{
			Template: fm.Template,
			Source:   source,
		},
		Slug:        fm.Slug,
		Title:       fm.Title,
		Description: fm.Description,
		Section:     fm.Section,
		Tags:        fm.Tags,
		Date:        fm.Date,
		Updated:     fm.Updated,
		PubDate:     firstNonzero(fm.Updated, fm.Date, time.Now()),
		Params:      fm.Params,
		LiteParams:  fm.LiteParams,
		Headers:     fm.Headers,
		RSS:         fm.RSS,
		Sitemap:     fm.Sitemap,
		Body:        template.HTML(body),
		Featured:    fm.Featured,
		Draft:       fm.Draft,
	}, nil
}

// buildMD builds a page from a markdown file
func buildMD(path string, md gm.Markdown) (*Frontmatter, string, error) {
	doc, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, "", err
	}

	var buf strings.Builder
	if err := md.Convert(body, &buf); err != nil {
		return nil, "", err
	}

	return fm, buf.String(), nil
}

// buildTOML builds a page from a TOML file
func buildTOML(path string) (*Frontmatter, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	fm := new(Frontmatter)

	if _, err := toml.NewDecoder(file).Decode(fm); err != nil {
		return nil, "", err
	}

	return fm, fm.Body, nil
}

// buildYaml builds a page from a YAML file
func buildYaml(path string) (*Frontmatter, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	fm := new(Frontmatter)

	if err := yaml.NewDecoder(file).Decode(fm); err != nil {
		return nil, "", err
	}

	return fm, fm.Body, nil
}

// buildJSON builds a page from a JSON file
func buildJSON(path string) (*Frontmatter, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	fm := new(Frontmatter)

	if err := json.NewDecoder(file).Decode(fm); err != nil {
		return nil, "", err
	}

	return fm, fm.Body, nil
}
