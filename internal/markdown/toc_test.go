package markdown

import (
	"strings"
	"testing"

	"github.com/olimci/shizuka/internal/config"
)

func TestRenderCollectsToC(t *testing.T) {
	md := Build(config.ConfigContentMarkdown{
		Parser: config.ConfigMarkdownParser{
			AutoHeadingID: true,
			Attribute:     true,
		},
	}, Options{})

	doc, err := Render(md, "test.md", "# Intro\n\n## Details {#custom}\n\n---\n\n### Later")
	if err != nil {
		t.Fatal(err)
	}

	if len(doc.ToC) != 3 {
		t.Fatalf("expected 3 ToC entries, got %d", len(doc.ToC))
	}

	assertToCEntry(t, doc.ToC[0], 1, "intro", "Intro")
	assertToCEntry(t, doc.ToC[1], 2, "custom", "Details")
	assertToCEntry(t, doc.ToC[2], 3, "later", "Later")

	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	if !strings.Contains(string(doc.Body), `id="intro"`) {
		t.Fatalf("body did not include generated heading id:\n%s", doc.Body)
	}
}

func assertToCEntry(t *testing.T, entry ToCEntry, level int, id, text string) {
	t.Helper()
	if entry.Level != level || entry.ID != id || entry.Text != text {
		t.Fatalf("unexpected ToC entry: got %#v, want level=%d id=%q text=%q", entry, level, id, text)
	}
}
