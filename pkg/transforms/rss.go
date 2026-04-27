package transforms

import (
	"encoding/xml"
	"slices"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/utils/set"
)

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	sortDate    time.Time
}

type RSSTemplateData struct {
	Title       string
	Link        string
	Description string
	BuildDate   string
	Items       []RSSItem
}

func BuildRSS(pages []*Page, site *Site, cfg *config.ConfigRSS) RSSTemplateData {
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

		pubDate := firstNonzero(page.PubDate, page.Updated, page.Created, time.Now())

		link := page.Canon
		if link == "" {
			link = page.URLPath
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
		return b.sortDate.Compare(a.sortDate)
	})

	return RSSTemplateData{
		Title:       site.Title,
		Link:        site.URL,
		Description: site.Description,
		BuildDate:   site.Meta.BuildTime.Format(time.RFC1123Z),
		Items:       items,
	}
}

func RenderRSS(data RSSTemplateData) (string, error) {
	doc := rssDocument{
		Version: "2.0",
		Channel: rssChannel{
			Title:         data.Title,
			Link:          data.Link,
			Description:   data.Description,
			LastBuildDate: data.BuildDate,
			Items:         data.Items,
		},
	}

	out, err := xml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return xml.Header + string(out), nil
}
