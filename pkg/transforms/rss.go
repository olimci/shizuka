package transforms

import (
	"html/template"
	"slices"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/utils/lazy"
	"github.com/olimci/shizuka/pkg/utils/set"
)

var RSSTemplate = lazy.New(func() *template.Template {
	return template.Must(template.New("rss").Parse(
		`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>{{ .Title }}</title>
<link>{{ .Link }}</link>
<description>{{ .Description }}</description>
<lastBuildDate>{{ .BuildDate }}</lastBuildDate>
{{- range .Items }}
<item>
<title>{{ .Title }}</title>
<link>{{ .Link }}</link>
<guid>{{ .GUID }}</guid>
<description>{{ .Description }}</description>
<pubDate>{{ .PubDate }}</pubDate>
</item>
{{- end }}
</channel>
</rss>
`))
})

type RSSItem struct {
	Title       string
	Link        string
	Description string
	GUID        string
	PubDate     string
	sortDate    time.Time
}

type RSSTemplateData struct {
	Title       string
	Link        string
	Description string
	BuildDate   string
	Items       []RSSItem
}

func BuildRSS(pages []*Page, site *Site, cfg *config.ConfigStepRSS) RSSTemplateData {
	sectionFilter := set.FromSlice(cfg.Sections)
	items := make([]RSSItem, 0, len(pages))
	for _, page := range pages {
		if !cfg.IncludeDrafts && page.Draft {
			continue
		}
		if !page.RSS.Include {
			continue
		}
		if !sectionFilter.Has(page.Section) {
			continue
		}

		pubDate := firstNonzero(page.Date, page.Updated, time.Now())

		link := page.Canon
		if link == "" {
			link = page.Meta.URLPath
		}

		items = append(items, RSSItem{
			Title:       firstNonzero(page.RSS.Title, page.Title),
			Link:        link,
			Description: firstNonzero(page.RSS.Description, page.Description),
			GUID:        firstNonzero(page.RSS.GUID, link),
			PubDate:     pubDate.Format(time.RFC1123Z),
			sortDate:    pubDate,
		})
	}

	slices.SortFunc(items, func(a, b RSSItem) int {
		return a.sortDate.Compare(b.sortDate)
	})

	return RSSTemplateData{
		Title:       site.Title,
		Link:        site.URL,
		Description: site.Description,
		BuildDate:   site.Meta.BuildTime.Format(time.RFC1123Z),
		Items:       items,
	}
}
