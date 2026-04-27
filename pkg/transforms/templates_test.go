package transforms

import (
	"html/template"
	"slices"
	"testing"
	"time"
)

func TestTemplateFunctions(t *testing.T) {
	funcs := DefaultTemplateFuncs()
	for _, key := range []string{"slugify", "asset", "dict", "merge", "asPages", "asPage"} {
		if funcs[key] == nil {
			t.Fatalf("DefaultTemplateFuncs() missing %q", key)
		}
	}

	if got := TemplateFuncDateFmt(time.RFC3339, time.Time{}); got != "" {
		t.Fatalf("TemplateFuncDateFmt(zero) = %q, want empty", got)
	}
	if got := TemplateFuncDateFmt("2006-01-02", time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC)); got != "2024-03-04" {
		t.Fatalf("TemplateFuncDateFmt() = %q, want %q", got, "2024-03-04")
	}
	if got := TemplateFuncSlugify("  Hello, World / Again  "); got != "hello-world-again" {
		t.Fatalf("TemplateFuncSlugify() = %q, want %q", got, "hello-world-again")
	}
	if got := TemplateFuncUniq([]string{"a", "b", "a", "c"}); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("TemplateFuncUniq() = %#v, want deduped order", got)
	}
	if got := TemplateFuncDefault("fallback", ""); got != "fallback" {
		t.Fatalf("TemplateFuncDefault() = %v, want fallback", got)
	}

	page := Page{
		Assets: map[string]*PageAsset{
			"images/logo.png": {URL: "/assets/logo.png"},
		},
	}
	if got := TemplateFuncAsset("images/logo.png", page); got != "/assets/logo.png" {
		t.Fatalf("TemplateFuncAsset() = %q, want %q", got, "/assets/logo.png")
	}
	if got := TemplateFuncAssetMeta("/images/logo.png", page); got == nil || got.URL != "/assets/logo.png" {
		t.Fatalf("TemplateFuncAssetMeta() = %#v, want matching asset", got)
	}
	if got := TemplateFuncRawHTML("<b>ok</b>"); got != template.HTML("<b>ok</b>") {
		t.Fatalf("TemplateFuncRawHTML() = %q, want raw HTML", got)
	}
}

func TestTemplateDictMergeFirstAsPagesAndAsPage(t *testing.T) {
	dict, err := TemplateFuncDict("a", 1, "b", 2)
	if err != nil {
		t.Fatalf("TemplateFuncDict() error = %v", err)
	}
	if dict["a"] != 1 || dict["b"] != 2 {
		t.Fatalf("TemplateFuncDict() = %#v, want keys", dict)
	}
	if _, err := TemplateFuncDict("a", 1, 2); err == nil {
		t.Fatal("TemplateFuncDict(odd args) error = nil, want error")
	}

	merged := TemplateFuncMerge(map[string]any{"a": 1}, map[string]any{"a": 2, "b": 3})
	if merged["a"] != 2 || merged["b"] != 3 {
		t.Fatalf("TemplateFuncMerge() = %#v, want later maps to win", merged)
	}

	page := &Page{Title: "Hello"}
	query := &QueryResult{
		Rows:  []map[string]any{{"title": "Hello"}},
		Pages: []*Page{page},
	}
	if got := TemplateFuncFirst(query); got.(map[string]any)["title"] != "Hello" {
		t.Fatalf("TemplateFuncFirst(query) = %#v, want first row", got)
	}
	if got := TemplateFuncFirst([]int{4, 5}); got.(int) != 4 {
		t.Fatalf("TemplateFuncFirst(slice) = %#v, want 4", got)
	}

	pages, err := TemplateFuncAsPages(query)
	if err != nil || len(pages) != 1 || pages[0] != page {
		t.Fatalf("TemplateFuncAsPages() = %#v, %v; want one page", pages, err)
	}
	firstPage, err := TemplateFuncAsPage(query)
	if err != nil || firstPage != page {
		t.Fatalf("TemplateFuncAsPage() = %#v, %v; want first page", firstPage, err)
	}

	_, err = TemplateFuncAsPages(&QueryResult{Rows: []map[string]any{{"title": "Hello"}}})
	if err == nil {
		t.Fatal("TemplateFuncAsPages(no pages) error = nil, want error")
	}
}
