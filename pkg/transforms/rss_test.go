package transforms

import (
	"strings"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
)

func TestRenderRSSProducesValidXMLAndNewestFirst(t *testing.T) {
	t.Parallel()

	oldTime := time.Date(2026, 1, 14, 17, 3, 27, 0, time.UTC)
	newTime := time.Date(2026, 4, 14, 18, 48, 24, 0, time.FixedZone("MDT", -6*60*60))

	data := BuildRSS([]*Page{
		{
			Title:       "Older",
			Description: "first & oldest",
			Section:     "posts",
			Canon:       "https://example.com/posts/older/",
			Created:     oldTime,
			RSS:         RSSMeta{Include: true},
		},
		{
			Title:       "Newer",
			Description: "second < newest",
			Section:     "posts",
			Canon:       "https://example.com/posts/newer/",
			Created:     newTime,
			RSS:         RSSMeta{Include: true},
		},
	}, &Site{
		Title:       "oli's site",
		URL:         "https://example.com/",
		Description: "Personal <site>",
		Meta:        SiteMeta{BuildTime: newTime},
	}, &config.ConfigRSS{
		Sections:      []string{"posts"},
		IncludeDrafts: true,
	})

	doc, err := RenderRSS(data)
	if err != nil {
		t.Fatalf("RenderRSS failed: %v", err)
	}

	if !strings.HasPrefix(doc, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Fatalf("expected XML declaration, got %q", doc)
	}
	if strings.Contains(doc, "&lt;?xml") {
		t.Fatalf("expected literal XML declaration, got %q", doc)
	}
	if !strings.Contains(doc, "<title>oli&#39;s site</title>") {
		t.Fatalf("expected XML-escaped title, got %q", doc)
	}
	if !strings.Contains(doc, "<description>Personal &lt;site&gt;</description>") {
		t.Fatalf("expected XML-escaped description, got %q", doc)
	}

	newerIdx := strings.Index(doc, "<title>Newer</title>")
	olderIdx := strings.Index(doc, "<title>Older</title>")
	if newerIdx < 0 || olderIdx < 0 {
		t.Fatalf("expected both items in output, got %q", doc)
	}
	if newerIdx > olderIdx {
		t.Fatalf("expected newest item first, got %q", doc)
	}
}
