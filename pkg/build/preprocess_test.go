package build

import (
	"strings"
	"testing"

	"github.com/olimci/shizuka/pkg/transforms"
)

func TestPreprocessMarkdownUsesIndexedPageLookup(t *testing.T) {
	t.Parallel()

	source := &transforms.Page{
		Title:       "Source",
		SourcePath:  "content/notes/source.md",
		ContentPath: "notes/source.md",
		URLPath:     "notes/source",
		Source: transforms.PageSource{
			Format:  transforms.PageSourceFormatMarkdown,
			RawBody: "[[posts/exact]] [[base]] [[dup]] [Exact](/posts/exact/) [Base](/articles/base-article/) [Dup](dup)",
		},
		Assets: map[string]*transforms.PageAsset{},
	}
	exact := &transforms.Page{
		Title:       "Exact",
		SourcePath:  "content/posts/exact.md",
		ContentPath: "posts/exact.md",
		URLPath:     "posts/exact",
		Slug:        "posts/exact",
	}
	base := &transforms.Page{
		Title:       "Base Article",
		SourcePath:  "content/articles/base.md",
		ContentPath: "articles/base.md",
		URLPath:     "articles/base-article",
	}
	dupA := &transforms.Page{
		Title:       "Dup A",
		SourcePath:  "content/posts/dup.md",
		ContentPath: "posts/dup.md",
		URLPath:     "posts/dup-a",
	}
	dupB := &transforms.Page{
		Title:       "Dup B",
		SourcePath:  "content/notes/dup.md",
		ContentPath: "notes/dup.md",
		URLPath:     "notes/dup-b",
	}

	body, links := preprocessMarkdown(source, newPageLookup([]*transforms.Page{source, exact, base, dupA, dupB}), true)

	if !strings.Contains(body, "[Exact](/posts/exact/)") {
		t.Fatalf("expected exact wikilink rewrite, got %q", body)
	}
	if !strings.Contains(body, "[Base Article](/articles/base-article/)") {
		t.Fatalf("expected basename wikilink rewrite, got %q", body)
	}
	if strings.Contains(body, "[[dup]]") == false {
		t.Fatalf("expected ambiguous wikilink to stay unresolved, got %q", body)
	}

	if len(links) != 4 {
		t.Fatalf("expected 4 resolved links, got %d (%#v)", len(links), links)
	}
	for _, link := range links {
		if link.Source != source {
			t.Fatalf("expected link source to be populated, got %#v", link)
		}
		if link.RawTarget == "dup" || link.Target == nil {
			t.Fatalf("expected ambiguous link to remain unresolved and not be recorded, got %#v", link)
		}
	}
}
