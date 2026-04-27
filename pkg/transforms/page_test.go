package transforms

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildPageParsesMarkdownAndDataSources(t *testing.T) {
	root := t.TempDir()

	markdownSource := filepath.Join(root, "content", "posts", "hello.md")
	if err := os.MkdirAll(filepath.Dir(markdownSource), 0o755); err != nil {
		t.Fatal(err)
	}
	markdown := `---
slug: hello
url_path: posts/hello
title: Hello
section: posts
created: 2024-01-02T03:04:05Z
updated: 2024-02-03T04:05:06Z
template: post
aliases: ["/hi"]
tags: ["go", "tests"]
headers:
  X-Test: yes
params:
  author: oliver
featured: true
draft: true
weight: 7
---
Body content
`
	if err := os.WriteFile(markdownSource, []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}

	page, err := BuildPage(root, "content/posts/hello.md")
	if err != nil {
		t.Fatalf("BuildPage(markdown) error = %v", err)
	}
	if page.Source.Format != PageSourceFormatMarkdown || page.Source.OutputKind != PageOutputKindMarkdown {
		t.Fatalf("page.Source = %#v, want markdown metadata", page.Source)
	}
	if page.QueryPage != page {
		t.Fatal("page.QueryPage does not point to self")
	}
	if page.URLPath != "posts/hello" || page.Template != "post" || page.Title != "Hello" {
		t.Fatalf("page basics = %#v, want parsed frontmatter", page)
	}
	if page.PubDate != page.Updated {
		t.Fatalf("page.PubDate = %v, want updated time %v", page.PubDate, page.Updated)
	}
	if !page.Featured || !page.Draft || page.Weight != 7 {
		t.Fatalf("page flags = %#v, want featured draft weight", page)
	}
	if page.Source.FrontmatterDoc == nil || page.Source.FrontmatterDoc.Body != "Body content\n" {
		t.Fatalf("page.Source.FrontmatterDoc = %#v, want body", page.Source.FrontmatterDoc)
	}

	dataSource := filepath.Join(root, "content", "notes", "page.toml")
	if err := os.MkdirAll(filepath.Dir(dataSource), 0o755); err != nil {
		t.Fatal(err)
	}
	dataDoc := `
slug = "note"
url_path = "notes/note"
title = "Note"
body = "Rendered"
[queries.recent]
query = "select * from pages"
`
	if err := os.WriteFile(dataSource, []byte(dataDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	dataPage, err := BuildPage(root, "content/notes/page.toml")
	if err != nil {
		t.Fatalf("BuildPage(toml) error = %v", err)
	}
	if dataPage.Source.Format != PageSourceFormatTOML || dataPage.Source.MetadataKind != "data" {
		t.Fatalf("dataPage.Source = %#v, want TOML data source", dataPage.Source)
	}
	if dataPage.Source.DataDoc == nil || dataPage.Source.DataDoc.Body != "Rendered" {
		t.Fatalf("dataPage.Source.DataDoc = %#v, want parsed body", dataPage.Source.DataDoc)
	}
	if dataPage.PubDate.IsZero() {
		t.Fatal("dataPage.PubDate is zero")
	}
}

func TestBuildPageErrorsOnUnsupportedExtension(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "content", "image.txt")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := BuildPage(root, "content/image.txt")
	if !errors.Is(err, ErrUnsupportedContentType) {
		t.Fatalf("BuildPage() error = %v, want ErrUnsupportedContentType", err)
	}
}

func TestPageHelpersAndMetaClone(t *testing.T) {
	doc, body, err := parseHTMLPage([]byte("<h1>Hello</h1>"))
	if err != nil {
		t.Fatalf("parseHTMLPage() error = %v", err)
	}
	if doc.Body != "<h1>Hello</h1>" || body != "<h1>Hello</h1>" {
		t.Fatalf("parseHTMLPage() = %#v, %q; want raw body", doc, body)
	}

	payload := dataPagePayload{
		Aliases: []string{"a"},
		Tags:    []string{"go"},
		Queries: map[string]PageQueryDef{"recent": {Query: "select 1"}},
		Headers: map[string]string{"X-Test": "yes"},
		Params:  map[string]any{"author": "oliver"},
	}
	meta := payload.meta()
	payload.Aliases[0] = "changed"
	payload.Tags[0] = "changed"
	payload.Queries["recent"] = PageQueryDef{Query: "changed"}

	if meta.Aliases[0] != "a" || meta.Tags[0] != "go" || meta.Queries["recent"].Query != "select 1" {
		t.Fatalf("meta() = %#v, want cloned slice/map fields", meta)
	}

	page := &Page{BuildError: errors.New("boom")}
	if !page.HasError() {
		t.Fatal("HasError() = false, want true")
	}
	if (PageLink{}).Resolved() {
		t.Fatal("Resolved() = true, want false")
	}
	if !(PageLink{Target: &Page{}}).Resolved() {
		t.Fatal("Resolved() = false, want true")
	}
}

func TestParseJSONPage(t *testing.T) {
	doc := []byte(`{"slug":"hello","body":"Body","created":"2024-01-01T00:00:00Z"}`)
	page, err := parseJSONPage(doc)
	if err != nil {
		t.Fatalf("parseJSONPage() error = %v", err)
	}
	if page.Body != "Body" || page.Meta.Slug != "hello" {
		t.Fatalf("parseJSONPage() = %#v, want parsed body and slug", page)
	}
	if page.Meta.Created != time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("created = %v, want 2024-01-01T00:00:00Z", page.Meta.Created)
	}
}
