package transforms

import (
	"html/template"
	"maps"
	"slices"
	"time"

	"github.com/olimci/shizuka/pkg/frontmatter"
)

type Page struct {
	Git  PageGitMeta
	File PageFileMeta

	// TODO: we should be able to reduce this somewhat, since sourcePath includes contentPath. also notice how these fields are a subset of claim- could just put a claim struct here.
	SourcePath  string
	ContentPath string
	Path        string
	OutputPath  string
	Template    string

	Error error

	Canon  string
	Weight int

	Title       string
	Description string
	Section     string
	Slug        string
	Tags        []string

	RSS     frontmatter.RSSMeta
	Sitemap frontmatter.SitemapMeta
	Robots  frontmatter.RobotsMeta

	Created time.Time
	Updated time.Time
	PubDate time.Time

	Params map[string]any

	Preprocess string
	RawBody    string
	Body       template.HTML

	Featured bool
	Draft    bool
}

func (p *Page) CloneShallow() *Page {
	cloned := *p
	cloned.Tags = slices.Clone(p.Tags)
	cloned.Params = maps.Clone(p.Params)
	return &cloned
}

func (p *Page) ApplyFrontmatter(meta frontmatter.Frontmatter) {
	p.Template = meta.Template
	p.Weight = meta.Weight
	p.Title = meta.Title
	p.Description = meta.Description
	p.Section = meta.Section
	p.Slug = meta.Slug
	p.Tags = slices.Clone(meta.Tags)
	p.Created = meta.Created
	p.Updated = meta.Updated
	p.PubDate = firstNonzero(meta.Updated, meta.Created, time.Now())
	p.Params = maps.Clone(meta.Params)
	p.RSS = meta.RSS
	p.Sitemap = meta.Sitemap
	p.Robots = meta.Robots
	p.Featured = meta.Featured
	p.Draft = meta.Draft
}

type Site struct {
	Title       string
	Description string
	URL         string

	Params map[string]any

	Dev       bool
	Git       SiteGitMeta
	BuildTime time.Time
}

type PageTemplate struct {
	Error      error
	Page       PageTmpl
	Site       SiteTmpl
	Pagination *PaginationTmpl
}

type PaginationTmpl struct {
	Items   []any
	Page    int
	Pages   int
	Total   int
	PerPage int
	Prev    string
	Next    string
	Group   any
}

type SiteTmpl struct {
	Title       string
	Description string
	URL         string

	Params map[string]any

	Dev       bool
	Git       SiteGitMeta
	BuildTime time.Time
}

func (s *Site) Tmpl() SiteTmpl {
	if s == nil {
		return SiteTmpl{}
	}

	return SiteTmpl{
		Title:       s.Title,
		Description: s.Description,
		URL:         s.URL,
		Params:      s.Params,
		Dev:         s.Dev,
		Git:         s.Git,
		BuildTime:   s.BuildTime,
	}
}

type PageTmpl struct {
	Git  PageGitMeta
	File PageFileMeta

	Path string

	Canon  string
	Weight int

	Title       string
	Description string
	Section     string
	Slug        string
	Tags        []string

	Created time.Time
	Updated time.Time
	PubDate time.Time

	Params map[string]any

	Body template.HTML

	Featured bool
	Draft    bool
}

func (p *Page) Tmpl() PageTmpl {
	if p == nil {
		return PageTmpl{}
	}

	return PageTmpl{
		Git:         p.Git,
		File:        p.File,
		Path:        p.Path,
		Canon:       p.Canon,
		Weight:      p.Weight,
		Title:       p.Title,
		Description: p.Description,
		Section:     p.Section,
		Slug:        p.Slug,
		Tags:        p.Tags,
		Created:     p.Created,
		Updated:     p.Updated,
		PubDate:     p.PubDate,
		Params:      p.Params,
		Body:        p.Body,
		Featured:    p.Featured,
		Draft:       p.Draft,
	}
}
