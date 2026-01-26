package transforms

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"path"
	"strings"
	"time"
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

	Params  map[string]any
	Cascade map[string]any

	Headers map[string]string

	Body     template.HTML
	BodyRaw  []byte
	BodyType string

	Featured bool
	Draft    bool
}

// Lite returns a lite representation of the page
func (p *Page) Lite() *PageLite {
	params := maps.Clone(p.Params)
	for k := range params {
		if !strings.HasPrefix(k, "_") {
			delete(params, k)
		}
	}

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
		Params:      params,
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

	Params map[string]any

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

// BuildPage builds a page from a file within the provided fs.FS.
func BuildPage(fsys fs.FS, source string) (*Page, error) {
	var (
		fm       *Frontmatter
		body     []byte
		err      error
		pageType string
	)

	// TODO: should extensions be able to create parsers here?
	switch ext := path.Ext(path.Base(source)); ext {
	case ".md":
		fm, body, err = buildMD(fsys, source)
		pageType = "markdown"
	case ".toml":
		fm, body, err = buildTOML(fsys, source)
		pageType = "data"
	case ".yaml", ".yml":
		fm, body, err = buildYaml(fsys, source)
		pageType = "data"
	case ".json":
		fm, body, err = buildJSON(fsys, source)
		pageType = "data"
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
		Headers:     fm.Headers,
		RSS:         fm.RSS,
		Sitemap:     fm.Sitemap,
		BodyRaw:     body,
		BodyType:    pageType,
		Featured:    fm.Featured,
		Draft:       fm.Draft,
	}, nil
}
