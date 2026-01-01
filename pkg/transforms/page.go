package transforms

import (
	"encoding/json"
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

type PageData struct {
	Page     Page
	Template string

	Source string
	Target string
}

type Page struct {
	Slug string

	Title       string
	Description string
	Section     string
	Tags        []string

	Date    time.Time
	Updated time.Time

	Params     map[string]any
	LiteParams map[string]any

	Body template.HTML

	Featured bool
	Draft    bool
}

func (p *Page) Lite() *PageLite {
	return &PageLite{
		Slug:        p.Slug,
		Title:       p.Title,
		Description: p.Description,
		Section:     p.Section,
		Tags:        p.Tags,
		Date:        p.Date,
		Updated:     p.Updated,
		LiteParams:  p.LiteParams,
		Featured:    p.Featured,
		Draft:       p.Draft,
	}
}

type PageLite struct {
	Slug string

	Title       string
	Description string
	Section     string
	Tags        []string

	Date    time.Time
	Updated time.Time

	LiteParams map[string]any

	Featured bool
	Draft    bool
}

// PageMeta contains metadata about a page's build context.
type PageMeta struct {
	Source   string // Original source file path
	Target   string // Output target path
	Template string // Template name used to render this page
}

// SiteMeta contains metadata about the site's build context.
type SiteMeta struct {
	BuildTime string // When the build started (RFC3339)
	Dev       bool   // Whether this is a dev build
}

type PageTemplate struct {
	Page     Page
	Site     Site
	Meta     PageMeta
	SiteMeta SiteMeta
}

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

func BuildPage(src string, md gm.Markdown) (*PageData, error) {
	var (
		fm   *Frontmatter
		body string
		err  error
	)

	switch ext := filepath.Ext(filepath.Base(src)); ext {
	case ".md":
		fm, body, err = buildMD(src, md)
	case ".toml":
		fm, body, err = buildTOML(src)
	case ".yaml", ".yml":
		fm, body, err = buildYaml(src)
	case ".json":
		fm, body, err = buildJSON(src)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	if err != nil {
		return nil, err
	}

	return &PageData{
		Page: Page{
			Slug:        fm.Slug,
			Title:       fm.Title,
			Description: fm.Description,
			Section:     fm.Section,
			Tags:        fm.Tags,
			Date:        fm.Date,
			Updated:     fm.Updated,
			Params:      fm.Params,
			LiteParams:  fm.LiteParams,
			Body:        template.HTML(body),
			Featured:    fm.Featured,
			Draft:       fm.Draft,
		},
		Template: fm.Template,
		Source:   src,
	}, nil
}
