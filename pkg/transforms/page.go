package transforms

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"mime"
	"path"
	"slices"
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
	Git  PageGitMeta

	Source PageSource
	Bundle PageBundle

	Slug    string
	Canon   string
	Aliases []string
	Weight  int

	Title       string
	Description string
	Section     string
	Tags        []string

	RSS     RSSMeta
	Sitemap SitemapMeta

	Date    time.Time
	Updated time.Time
	PubDate time.Time

	Params map[string]any

	Headers map[string]string

	Body template.HTML

	Featured bool
	Draft    bool
}

// Lite returns a lite representation of the page
func (p *Page) Lite() *PageLite {
	params := maps.Clone(p.Params)
	for k := range params {
		if strings.HasPrefix(k, "_") {
			delete(params, k)
		}
	}

	return &PageLite{
		Git:         p.Git,
		Slug:        p.Slug,
		Canon:       p.Canon,
		Aliases:     slices.Clone(p.Aliases),
		Weight:      p.Weight,
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
	Git PageGitMeta

	Slug    string
	Canon   string
	Aliases []string
	Weight  int

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

type PageSourceKind string

const (
	PageSourceKindMarkdown   PageSourceKind = "markdown"
	PageSourceKindHTML       PageSourceKind = "html"
	PageSourceKindStructured PageSourceKind = "structured"
)

type PageSource struct {
	Kind     PageSourceKind
	Ext      string
	Doc      string
	Body     string
	BodyHTML bool
}

type PageBundle struct {
	Assets map[string]*PageAsset
}

type PageAsset struct {
	Key        string
	Source     string
	Target     string
	URL        string
	Hash       string
	Size       int64
	MediaType  string
	Standalone bool
}

// PageTemplate is the struct from which page templates are built
type PageTemplate struct {
	Page  Page
	Site  Site
	Error error
}

// BuildPageFS builds a page from a file within the provided fs.FS.
func BuildPageFS(fsys fs.FS, source string, _ gm.Markdown) (*Page, error) {
	var (
		fm   *Frontmatter
		body string
		doc  string
		kind PageSourceKind
		html bool
		err  error
	)

	switch ext := path.Ext(path.Base(source)); ext {
	case ".md":
		fm, body, doc, err = buildMDFromFS(fsys, source)
		kind = PageSourceKindMarkdown
	case ".toml":
		fm, body, doc, err = buildTOMLFromFS(fsys, source)
		kind = PageSourceKindStructured
		html = true
	case ".yaml", ".yml":
		fm, body, doc, err = buildYamlFromFS(fsys, source)
		kind = PageSourceKindStructured
		html = true
	case ".json":
		fm, body, doc, err = buildJSONFromFS(fsys, source)
		kind = PageSourceKindStructured
		html = true
	case ".html":
		fm, body, doc, err = buildHTMLFromFS(fsys, source)
		kind = PageSourceKindHTML
		html = true
	default:
		return nil, fmt.Errorf("%w %q", ErrUnsupportedContentType, ext)
	}

	if err != nil {
		return nil, err
	}

	return &Page{
		Meta: PageMeta{
			Template: fm.Template,
			Source:   source,
			URLPath:  fm.URLPath,
		},
		Source: PageSource{
			Kind:     kind,
			Ext:      path.Ext(path.Base(source)),
			Doc:      doc,
			Body:     body,
			BodyHTML: html,
		},
		Bundle: PageBundle{
			Assets: make(map[string]*PageAsset),
		},
		Slug:        fm.Slug,
		Aliases:     slices.Clone(fm.Aliases),
		Weight:      fm.Weight,
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
		Body:        "",
		Featured:    fm.Featured,
		Draft:       fm.Draft,
	}, nil
}

func buildMDFromFS(fsys fs.FS, path string) (*Frontmatter, string, string, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, "", "", err
	}

	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, "", "", err
	}

	return fm, string(body), string(doc), nil
}

func buildTOMLFromFS(fsys fs.FS, path string) (*Frontmatter, string, string, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, "", "", err
	}

	fm := new(Frontmatter)

	if _, err := toml.Decode(string(doc), fm); err != nil {
		return nil, "", "", fmt.Errorf("TOML page data: %w", err)
	}

	return fm, fm.Body, string(doc), nil
}

func buildYamlFromFS(fsys fs.FS, path string) (*Frontmatter, string, string, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, "", "", err
	}

	fm := new(Frontmatter)

	if err := yaml.NewDecoder(strings.NewReader(string(doc))).Decode(fm); err != nil {
		return nil, "", "", fmt.Errorf("YAML page data: %w", err)
	}

	return fm, fm.Body, string(doc), nil
}

func buildJSONFromFS(fsys fs.FS, path string) (*Frontmatter, string, string, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, "", "", err
	}

	fm := new(Frontmatter)

	if err := json.NewDecoder(strings.NewReader(string(doc))).Decode(fm); err != nil {
		return nil, "", "", fmt.Errorf("JSON page data: %w", err)
	}

	return fm, fm.Body, string(doc), nil
}

func buildHTMLFromFS(fsys fs.FS, path string) (*Frontmatter, string, string, error) {
	doc, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, "", "", err
	}

	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, "", "", err
	}

	return fm, string(body), string(doc), nil
}

func PageAssetMediaType(source string) string {
	typ := mime.TypeByExtension(strings.ToLower(path.Ext(source)))
	if typ == "" {
		return "application/octet-stream"
	}
	return typ
}
