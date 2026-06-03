package frontmatter

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestExtractWithDefaultsAppliesSectionDefaultsBeforeDocumentFields(t *testing.T) {
	globalCreated := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	sectionUpdated := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	defaults := Defaults{
		Title:    "global title",
		Tags:     []string{"global"},
		Created:  globalCreated,
		Template: "page",
		RSS: RSSMeta{
			Include: true,
		},
	}
	bySection := map[string]Defaults{
		"posts": {
			Title:    "section title",
			Tags:     []string{"section"},
			Updated:  sectionUpdated,
			Template: "post",
			Sitemap: SitemapMeta{
				Include:  true,
				Priority: 0.8,
			},
		},
	}

	fm, body, err := ExtractWithDefaults([]byte("+++\nsection = \"posts\"\ntitle = \"doc title\"\n+++\nHello"), "pages", defaults, bySection)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "Hello" {
		t.Fatalf("body = %q, want Hello", body)
	}
	if fm.Section != "posts" {
		t.Fatalf("section = %q, want posts", fm.Section)
	}
	if fm.Title != "doc title" {
		t.Fatalf("title = %q, want doc title", fm.Title)
	}
	if !slices.Equal(fm.Tags, []string{"section"}) {
		t.Fatalf("tags = %#v, want section default tags", fm.Tags)
	}
	if !fm.Updated.Equal(sectionUpdated) {
		t.Fatalf("updated = %s, want %s", fm.Updated, sectionUpdated)
	}
	if fm.Template != "post" {
		t.Fatalf("template = %q, want post", fm.Template)
	}
	if !fm.Sitemap.Include || fm.Sitemap.Priority != 0.8 {
		t.Fatalf("sitemap = %#v, want section defaults", fm.Sitemap)
	}
	if fm.RSS.Include {
		t.Fatalf("rss include = true, want section defaults to replace global defaults")
	}
}

func TestExtractJSONFrontmatterSkipsCommentsAndStringBraces(t *testing.T) {
	doc := []byte("{\n  // comment with }\n  \"title\": \"{braced}\",\n  \"params\": {\"kind\": \"note\"}\n}\nBody")

	fm, body, err := Extract(doc)
	if err != nil {
		t.Fatal(err)
	}

	if fm.Title != "{braced}" {
		t.Fatalf("title = %q, want {braced}", fm.Title)
	}
	if fm.Params["kind"] != "note" {
		t.Fatalf("params = %#v, want kind note", fm.Params)
	}
	if string(body) != "Body" {
		t.Fatalf("body = %q, want Body", body)
	}
}

func TestExtractReturnsFallbackBodyOnParseError(t *testing.T) {
	doc := []byte("+++\ntitle = \n+++\nBody")

	fm, body, err := Extract(doc)
	if !errors.Is(err, ErrParse) {
		t.Fatalf("err = %v, want ErrParse", err)
	}
	if fm != nil {
		t.Fatalf("frontmatter = %#v, want nil", fm)
	}
	if string(body) != string(doc) {
		t.Fatalf("body = %q, want original document", body)
	}
}

func TestExtractWithoutFrontmatterUsesDefaultSection(t *testing.T) {
	defaults := Defaults{
		Title: "Default",
		Tags:  []string{"default"},
	}

	fm, body, err := ExtractWithDefaults([]byte("Body"), "pages", defaults, nil)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "Body" {
		t.Fatalf("body = %q, want Body", body)
	}
	if fm.Section != "pages" || fm.Title != "Default" || !slices.Equal(fm.Tags, []string{"default"}) {
		t.Fatalf("frontmatter = %#v, want default section and fields", fm)
	}
}
