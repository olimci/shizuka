package transforms

import (
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
)

func TestBuildRSSAppliesFilteringFallbacksAndOrdering(t *testing.T) {
	site := &Site{
		Title:       "Site",
		Description: "Desc",
		URL:         "https://example.com",
		Meta: SiteMeta{
			BuildTime: time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC),
		},
	}
	cfg := &config.ConfigRSS{Sections: []string{"posts"}, IncludeDrafts: false}
	pages := []*Page{
		{
			Title:   "Second",
			Section: "posts",
			Canon:   "https://example.com/second/",
			RSS:     RSSMeta{Include: true},
			PubDate: time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			Title:       "First",
			Description: "Page description",
			Section:     "posts",
			URLPath:     "/first",
			RSS: RSSMeta{
				Include: true,
				Title:   "RSS First",
			},
			Created: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Title:   "Draft",
			Section: "posts",
			Draft:   true,
			RSS:     RSSMeta{Include: true},
		},
		{
			Title:   "Ignored section",
			Section: "notes",
			RSS:     RSSMeta{Include: true},
		},
	}

	data := BuildRSS(pages, site, cfg)

	if data.Title != "Site" || data.Link != "https://example.com" || data.Description != "Desc" {
		t.Fatalf("BuildRSS() site metadata = %#v, want site values", data)
	}
	if len(data.Items) != 2 {
		t.Fatalf("len(data.Items) = %d, want 2", len(data.Items))
	}
	if data.Items[0].Title != "Second" || data.Items[1].Title != "RSS First" {
		t.Fatalf("item order = %#v, want newest first", data.Items)
	}
	if data.Items[1].Link != "/first" || data.Items[1].GUID != "/first" || data.Items[1].Description != "Page description" {
		t.Fatalf("fallback item fields = %#v, want URLPath/GUID/description fallbacks", data.Items[1])
	}
}

func TestBuildSitemapFiltersAndSorts(t *testing.T) {
	site := &Site{URL: "https://example.com"}
	cfg := &config.ConfigSitemap{IncludeDrafts: false}
	pages := []*Page{
		{
			Canon: "https://example.com/b/",
			Sitemap: SitemapMeta{
				Include:    true,
				ChangeFreq: "weekly",
				Priority:   0.5,
			},
			Updated: time.Date(2024, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			Sitemap: SitemapMeta{
				Include:  true,
				Priority: 1,
			},
			Created: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Canon: "https://example.com/draft/",
			Sitemap: SitemapMeta{
				Include: true,
			},
			Draft: true,
		},
	}

	data := BuildSitemap(pages, site, cfg)

	if len(data.Items) != 2 {
		t.Fatalf("len(data.Items) = %d, want 2", len(data.Items))
	}
	if data.Items[0].Loc != "https://example.com" || data.Items[1].Loc != "https://example.com/b/" {
		t.Fatalf("sorted sitemap items = %#v, want site URL fallback then canonical URL", data.Items)
	}
	if data.Items[1].ChangeFreq != "weekly" || data.Items[1].Priority != "0.50" {
		t.Fatalf("sitemap metadata = %#v, want formatted values", data.Items[1])
	}
}
