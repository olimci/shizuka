package transforms

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/frontmatter"
)

func TestBuildRSSFiltersSectionsDraftsAndSortsNewestFirst(t *testing.T) {
	older := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	site := &Site{
		Title:       "Site",
		Description: "Desc",
		URL:         "https://example.com",
		BuildTime:   newer,
	}
	pages := []*Page{
		rssPage("Old", "posts", older, false),
		rssPage("Draft", "posts", newer.Add(time.Hour), true),
		rssPage("Page", "pages", newer, false),
		rssPage("New", "posts", newer, false),
	}

	data := BuildRSS(pages, site, &config.ConfigRSS{Sections: []string{"posts"}})

	if len(data.Items) != 2 {
		t.Fatalf("items = %#v, want 2 post items", data.Items)
	}
	if data.Items[0].Title != "New" || data.Items[1].Title != "Old" {
		t.Fatalf("items = %#v, want newest posts first", data.Items)
	}
	if data.Title != "Site" || data.Link != "https://example.com" || data.Description != "Desc" {
		t.Fatalf("channel data = %#v, want site fields", data)
	}
}

func TestRenderRSSProducesXML(t *testing.T) {
	out, err := RenderRSS(RSSTemplateData{
		Title:       "Site",
		Link:        "https://example.com",
		Description: "Desc",
		BuildDate:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC1123Z),
		Items: []RSSItem{{
			Title: "Post",
			Link:  "https://example.com/post/",
			GUID:  "post-guid",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, xml.Header) {
		t.Fatalf("rss did not start with XML header:\n%s", out)
	}
	if !strings.Contains(out, `<rss version="2.0">`) || !strings.Contains(out, `<guid>post-guid</guid>`) {
		t.Fatalf("rss missing expected XML:\n%s", out)
	}
}

func TestBuildSitemapFiltersAndSorts(t *testing.T) {
	created := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	pages := []*Page{
		sitemapPage("https://example.com/z/", created, false, true),
		sitemapPage("https://example.com/draft/", created, true, true),
		sitemapPage("https://example.com/a/", created, false, true),
		sitemapPage("https://example.com/hidden/", created, false, false),
	}

	data := BuildSitemap(pages, &Site{URL: "https://example.com"}, &config.ConfigSitemap{})

	if len(data.Items) != 2 {
		t.Fatalf("items = %#v, want 2", data.Items)
	}
	if data.Items[0].Loc != "https://example.com/a/" || data.Items[1].Loc != "https://example.com/z/" {
		t.Fatalf("items = %#v, want sorted included non-drafts", data.Items)
	}
	if data.Items[0].LastMod != created.Format(time.RFC3339) {
		t.Fatalf("lastmod = %q, want created time", data.Items[0].LastMod)
	}
}

func TestBuildRobotsCombinesConfiguredGroupsPageDisallowsAndSitemap(t *testing.T) {
	data := BuildRobots(
		[]*Page{
			{Path: "/private/", Robots: frontmatter.RobotsMeta{Disallow: true}},
			{Path: "/draft/", Draft: true, Robots: frontmatter.RobotsMeta{Disallow: true}},
			{Path: "/public/"},
		},
		&Site{URL: "https://example.com/base/"},
		&config.ConfigRobots{
			IncludeSitemap: true,
			Groups: []config.RobotsGroup{{
				UserAgents: []string{"Googlebot"},
				Allow:      []string{"/"},
			}},
		},
		&config.ConfigSitemap{Path: "/sitemap.xml"},
	)

	out := RenderRobots(data)
	want := "User-agent: Googlebot\nAllow: /\n\nUser-agent: *\nDisallow: /private/\n\nSitemap: https://example.com/sitemap.xml\n"
	if out != want {
		t.Fatalf("robots = %q, want %q", out, want)
	}
}

func rssPage(title, section string, created time.Time, draft bool) *Page {
	return &Page{
		Title:       title,
		Description: title + " desc",
		Section:     section,
		Path:        "/" + strings.ToLower(title) + "/",
		Canon:       "https://example.com/" + strings.ToLower(title) + "/",
		Created:     created,
		RSS: frontmatter.RSSMeta{
			Include: true,
		},
		Draft: draft,
	}
}

func sitemapPage(canon string, created time.Time, draft, include bool) *Page {
	return &Page{
		Canon:   canon,
		Created: created,
		Sitemap: frontmatter.SitemapMeta{
			Include:    include,
			ChangeFreq: "weekly",
			Priority:   0.7,
		},
		Draft: draft,
	}
}
