package transforms

import (
	"fmt"
	"html/template"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/config"
)

var SitemapTemplate = template.Must(template.New("sitemap").
	Parse(
		`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
{{- range .Items }}
<url>
<loc>{{ .Loc }}</loc>{{ if .LastMod }}
<lastmod>{{ .LastMod }}</lastmod>{{ end }}{{ if .ChangeFreq }}
<changefreq>{{ .ChangeFreq }}</changefreq>{{ end }}{{ if .Priority }}
<priority>{{ .Priority }}</priority>{{ end }}
</url>
{{- end }}
</urlset>
`))

type SitemapItem struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

type SitemapTemplateData struct {
	Items []SitemapItem
}

func BuildSitemap(pages []*Page, site *Site, cfg *config.ConfigStepSitemap) SitemapTemplateData {
	items := make([]SitemapItem, 0, len(pages))
	for _, page := range pages {
		if page.Meta.Err != nil {
			continue
		}
		if !cfg.IncludeDrafts && page.Draft {
			continue
		}
		if !page.Sitemap.Include {
			continue
		}

		lastMod := firstNonzero(page.Updated, page.Date, time.Now())

		loc, _ := url.JoinPath(site.URL, page.Meta.Claim.Canon)

		items = append(items, SitemapItem{
			Loc:        loc,
			LastMod:    lastMod.Format(time.RFC3339),
			ChangeFreq: page.Sitemap.ChangeFreq,
			Priority:   fmt.Sprintf("%.2f", page.Sitemap.Priority),
		})
	}

	slices.SortFunc(items, func(a, b SitemapItem) int {
		return strings.Compare(a.Loc, b.Loc)
	})

	return SitemapTemplateData{
		Items: items,
	}
}
