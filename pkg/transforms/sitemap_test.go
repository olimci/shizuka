package transforms

import (
	"strings"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
)

func TestRenderSitemapProducesValidXML(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 14, 18, 48, 24, 0, time.FixedZone("MDT", -6*60*60))

	data := BuildSitemap([]*Page{
		{
			Canon:   "https://example.com/posts/newer/",
			Created: now,
			Sitemap: SitemapMeta{Include: true, ChangeFreq: "weekly", Priority: 0.8},
		},
		{
			Canon:   "https://example.com/",
			Created: now,
			Sitemap: SitemapMeta{Include: true, ChangeFreq: "daily", Priority: 1.0},
		},
	}, &Site{
		URL: "https://example.com/",
	}, &config.ConfigSitemap{
		IncludeDrafts: true,
	})

	doc, err := RenderSitemap(data)
	if err != nil {
		t.Fatalf("RenderSitemap failed: %v", err)
	}

	if !strings.HasPrefix(doc, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Fatalf("expected XML declaration, got %q", doc)
	}
	if strings.Contains(doc, "&lt;?xml") {
		t.Fatalf("expected literal XML declaration, got %q", doc)
	}
	if !strings.Contains(doc, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">") {
		t.Fatalf("expected sitemap root element, got %q", doc)
	}

	rootIdx := strings.Index(doc, "<loc>https://example.com/</loc>")
	postIdx := strings.Index(doc, "<loc>https://example.com/posts/newer/</loc>")
	if rootIdx < 0 || postIdx < 0 {
		t.Fatalf("expected both URLs in output, got %q", doc)
	}
	if rootIdx > postIdx {
		t.Fatalf("expected loc values sorted ascending, got %q", doc)
	}
}
