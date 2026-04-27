package transforms

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/config"
)

type sitemapDocument struct {
	XMLName xml.Name      `xml:"urlset"`
	Xmlns   string        `xml:"xmlns,attr"`
	Items   []SitemapItem `xml:"url"`
}

type SitemapItem struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type SitemapTemplateData struct {
	Items []SitemapItem
}

func BuildSitemap(pages []*Page, site *Site, cfg *config.ConfigSitemap) SitemapTemplateData {
	items := make([]SitemapItem, 0, len(pages))
	for _, page := range pages {
		if !cfg.IncludeDrafts && page.Draft {
			continue
		}
		if !page.Sitemap.Include {
			continue
		}

		lastMod := firstNonzero(page.Updated, page.Created, time.Now())

		loc := page.Canon
		if loc == "" {
			loc = site.URL
		}

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

func RenderSitemap(data SitemapTemplateData) (string, error) {
	doc := sitemapDocument{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		Items: data.Items,
	}

	out, err := xml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return xml.Header + string(out), nil
}
