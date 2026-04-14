package transforms

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

var (
	ErrFailedToParsePage      = errors.New("failed to parse page")
	ErrUnsupportedContentType = errors.New("unsupported content type")
)

type Page struct {
	Parent   *Page
	Children []*Page
	Git      PageGitMeta

	Source PageSource

	SourcePath  string
	ContentPath string
	URLPath     string
	OutputPath  string
	Template    string
	BuildError  error

	Assets map[string]*PageAsset

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

	Links []PageLink

	Featured bool
	Draft    bool
}

type PageLink struct {
	RawTarget string
	Fragment  string
	Label     string
	Embed     bool
	Target    *Page
}

func (l PageLink) Resolved() bool {
	return l.Target != nil
}

func (p *Page) HasError() bool {
	return p != nil && p.BuildError != nil
}

type PageSourceFormat string

const (
	PageSourceFormatMarkdown PageSourceFormat = "markdown"
	PageSourceFormatHTML     PageSourceFormat = "html"
	PageSourceFormatTOML     PageSourceFormat = "toml"
	PageSourceFormatYAML     PageSourceFormat = "yaml"
	PageSourceFormatJSON     PageSourceFormat = "json"
)

type PageOutputKind string

const (
	PageOutputKindMarkdown PageOutputKind = "markdown"
	PageOutputKindHTML     PageOutputKind = "html"
)

type PageSource struct {
	Format         PageSourceFormat
	MetadataKind   string
	RawDocument    string
	RawBody        string
	Preprocessed   string
	OutputKind     PageOutputKind
	FrontmatterDoc *FrontmatterDoc
	DataDoc        *DataPageDoc
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

type FrontmatterDoc struct {
	Meta Frontmatter
	Body string
}

type DataPageDoc struct {
	Meta Frontmatter
	Body string
}

type dataPagePayload struct {
	Slug        string            `toml:"slug" yaml:"slug" json:"slug"`
	URLPath     string            `toml:"url_path" yaml:"url_path" json:"url_path"`
	Aliases     []string          `toml:"aliases" yaml:"aliases" json:"aliases"`
	Title       string            `toml:"title" yaml:"title" json:"title"`
	Description string            `toml:"description" yaml:"description" json:"description"`
	Section     string            `toml:"section" yaml:"section" json:"section"`
	Tags        []string          `toml:"tags" yaml:"tags" json:"tags"`
	Date        time.Time         `toml:"date" yaml:"date" json:"date"`
	Updated     time.Time         `toml:"updated" yaml:"updated" json:"updated"`
	RSS         RSSMeta           `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap     SitemapMeta       `toml:"sitemap" yaml:"sitemap" json:"sitemap"`
	Params      map[string]any    `toml:"params" yaml:"params" json:"params"`
	Headers     map[string]string `toml:"headers" yaml:"headers" json:"headers"`
	Template    string            `toml:"template" yaml:"template" json:"template"`
	Body        string            `toml:"body" yaml:"body" json:"body"`
	Featured    bool              `toml:"featured" yaml:"featured" json:"featured"`
	Draft       bool              `toml:"draft" yaml:"draft" json:"draft"`
	Weight      int               `toml:"weight" yaml:"weight" json:"weight"`
}

type PageTemplate struct {
	Page  Page
	Site  Site
	Error error
}

func BuildPage(sourceRoot, source string) (*Page, error) {
	source = path.Clean(strings.TrimSpace(source))
	doc, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(source)))
	if err != nil {
		return nil, err
	}

	var (
		sourceMeta PageSource
		meta       Frontmatter
	)

	switch ext := strings.ToLower(path.Ext(source)); ext {
	case ".md":
		docMeta, body, parseErr := parseMarkdownPage(doc)
		if parseErr != nil {
			return nil, parseErr
		}
		meta = docMeta.Meta
		sourceMeta = PageSource{
			Format:         PageSourceFormatMarkdown,
			MetadataKind:   "frontmatter",
			RawDocument:    string(doc),
			RawBody:        body,
			Preprocessed:   body,
			OutputKind:     PageOutputKindMarkdown,
			FrontmatterDoc: docMeta,
		}
	case ".html":
		docMeta, body, parseErr := parseHTMLPage(doc)
		if parseErr != nil {
			return nil, parseErr
		}
		meta = docMeta.Meta
		sourceMeta = PageSource{
			Format:         PageSourceFormatHTML,
			MetadataKind:   "frontmatter",
			RawDocument:    string(doc),
			RawBody:        body,
			Preprocessed:   body,
			OutputKind:     PageOutputKindHTML,
			FrontmatterDoc: docMeta,
		}
	case ".toml":
		dataDoc, parseErr := parseTOMLPage(doc)
		if parseErr != nil {
			return nil, parseErr
		}
		meta = dataDoc.Meta
		sourceMeta = PageSource{
			Format:       PageSourceFormatTOML,
			MetadataKind: "data",
			RawDocument:  string(doc),
			RawBody:      dataDoc.Body,
			Preprocessed: dataDoc.Body,
			OutputKind:   PageOutputKindHTML,
			DataDoc:      dataDoc,
		}
	case ".yaml", ".yml":
		dataDoc, parseErr := parseYAMLPage(doc)
		if parseErr != nil {
			return nil, parseErr
		}
		meta = dataDoc.Meta
		sourceMeta = PageSource{
			Format:       PageSourceFormatYAML,
			MetadataKind: "data",
			RawDocument:  string(doc),
			RawBody:      dataDoc.Body,
			Preprocessed: dataDoc.Body,
			OutputKind:   PageOutputKindHTML,
			DataDoc:      dataDoc,
		}
	case ".json":
		dataDoc, parseErr := parseJSONPage(doc)
		if parseErr != nil {
			return nil, parseErr
		}
		meta = dataDoc.Meta
		sourceMeta = PageSource{
			Format:       PageSourceFormatJSON,
			MetadataKind: "data",
			RawDocument:  string(doc),
			RawBody:      dataDoc.Body,
			Preprocessed: dataDoc.Body,
			OutputKind:   PageOutputKindHTML,
			DataDoc:      dataDoc,
		}
	default:
		return nil, fmt.Errorf("%w %q", ErrUnsupportedContentType, ext)
	}

	return &Page{
		Source:      sourceMeta,
		SourcePath:  source,
		URLPath:     meta.URLPath,
		Template:    meta.Template,
		Assets:      make(map[string]*PageAsset),
		Slug:        meta.Slug,
		Aliases:     slices.Clone(meta.Aliases),
		Weight:      meta.Weight,
		Title:       meta.Title,
		Description: meta.Description,
		Section:     meta.Section,
		Tags:        meta.Tags,
		Date:        meta.Date,
		Updated:     meta.Updated,
		PubDate:     firstNonzero(meta.Updated, meta.Date, time.Now()),
		Params:      meta.Params,
		Headers:     meta.Headers,
		Body:        "",
		Links:       nil,
		RSS:         meta.RSS,
		Sitemap:     meta.Sitemap,
		Featured:    meta.Featured,
		Draft:       meta.Draft,
	}, nil
}

func parseMarkdownPage(doc []byte) (*FrontmatterDoc, string, error) {
	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, "", err
	}
	if fm == nil {
		fm = &Frontmatter{}
	}

	return &FrontmatterDoc{Meta: *fm, Body: string(body)}, string(body), nil
}

func parseTOMLPage(doc []byte) (*DataPageDoc, error) {
	payload := new(dataPagePayload)

	if _, err := toml.Decode(string(doc), payload); err != nil {
		return nil, fmt.Errorf("TOML page data: %w", err)
	}

	return &DataPageDoc{Meta: payload.meta(), Body: payload.Body}, nil
}

func parseYAMLPage(doc []byte) (*DataPageDoc, error) {
	payload := new(dataPagePayload)

	if err := yaml.NewDecoder(strings.NewReader(string(doc))).Decode(payload); err != nil {
		return nil, fmt.Errorf("YAML page data: %w", err)
	}

	return &DataPageDoc{Meta: payload.meta(), Body: payload.Body}, nil
}

func parseJSONPage(doc []byte) (*DataPageDoc, error) {
	payload := new(dataPagePayload)

	if err := json.NewDecoder(strings.NewReader(string(doc))).Decode(payload); err != nil {
		return nil, fmt.Errorf("JSON page data: %w", err)
	}

	return &DataPageDoc{Meta: payload.meta(), Body: payload.Body}, nil
}

func parseHTMLPage(doc []byte) (*FrontmatterDoc, string, error) {
	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		return nil, "", err
	}
	if fm == nil {
		fm = &Frontmatter{}
	}

	return &FrontmatterDoc{Meta: *fm, Body: string(body)}, string(body), nil
}

func (p dataPagePayload) meta() Frontmatter {
	return Frontmatter{
		Slug:        p.Slug,
		URLPath:     p.URLPath,
		Aliases:     slices.Clone(p.Aliases),
		Title:       p.Title,
		Description: p.Description,
		Section:     p.Section,
		Tags:        slices.Clone(p.Tags),
		Date:        p.Date,
		Updated:     p.Updated,
		RSS:         p.RSS,
		Sitemap:     p.Sitemap,
		Params:      p.Params,
		Headers:     p.Headers,
		Template:    p.Template,
		Featured:    p.Featured,
		Draft:       p.Draft,
		Weight:      p.Weight,
	}
}
