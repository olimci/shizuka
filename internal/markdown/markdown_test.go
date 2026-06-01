package markdown

import (
	"strings"
	"testing"

	"github.com/olimci/shizuka/internal/config"
)

func TestHighlightingEmitsInlineColor(t *testing.T) {
	md := Build(config.ConfigContentMarkdown{
		Highlighting: &config.ConfigMarkdownHighlighting{
			Style: "monokai",
		},
	}, Options{})

	doc, err := Render(md, "test.md", "```go\nvar x = 1\n```")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(doc.Body), "color:") {
		t.Fatalf("highlighted markdown did not include inline color styles:\n%s", doc.Body)
	}
}

func TestHighlightingCanEmitClasses(t *testing.T) {
	md := Build(config.ConfigContentMarkdown{
		Highlighting: &config.ConfigMarkdownHighlighting{
			Classes: true,
			Style:   "monokai",
		},
	}, Options{})

	doc, err := Render(md, "test.md", "```go\nvar x = 1\n```")
	if err != nil {
		t.Fatal(err)
	}

	body := string(doc.Body)
	if !strings.Contains(body, `class="chroma`) {
		t.Fatalf("highlighted markdown did not include Chroma classes:\n%s", doc.Body)
	}
	if strings.Contains(body, "color:") {
		t.Fatalf("class-based highlighting included inline color styles:\n%s", doc.Body)
	}
}
