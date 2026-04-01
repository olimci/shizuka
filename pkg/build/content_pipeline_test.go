package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olimci/shizuka/pkg/config"
)

func TestBuildHTMLContentPage(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "shizuka.toml", `
[site]
title = "Test"
description = "Test site"
url = "https://example.com"
`)
	writeTestFile(t, root, "static/style.css", "")
	writeTestFile(t, root, "templates/page.tmpl", `{{ define "page" }}<html><body>{{ .Page.Body }}</body></html>{{ end }}`)
	writeTestFile(t, root, "content/about.html", `---
title: "About"
template: "page"
---
<p>Hello from HTML.</p>`)

	if err := Build(config.DefaultOptions().
		WithConfig(filepath.Join(root, "shizuka.toml")).
		WithOutput(filepath.Join(root, "dist"))); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	got := readTestFile(t, root, "dist/about/index.html")
	if !strings.Contains(got, "Hello from HTML.") {
		t.Fatalf("expected rendered HTML page body, got %q", got)
	}
}

func TestBuildBundlesAndRawMarkdown(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, root, "shizuka.toml", `
[site]
title = "Test"
description = "Test site"
url = "https://example.com"

[build.steps.content.bundle_assets]
enabled = true
output = "_assets"
mode = "fingerprinted"

[build.steps.content.raw]
markdown = true

[build.steps.content.markdown]
obsidian_links = true
`)
	writeTestFile(t, root, "static/style.css", "")
	writeTestFile(t, root, "templates/page.tmpl", `{{ define "page" }}<html><body>{{ .Page.Body }}<img src="{{ asset "hero.png" .Page }}"></body></html>{{ end }}`)
	writeTestFile(t, root, "content/post.md", `---
title: "Post"
template: "page"
---
![Hero](hero.png)
See [[about|About page]] and ![[hero.png]].
`)
	writeTestFile(t, root, "content/about.md", `---
title: "About"
template: "page"
---
About.`)
	writeTestFile(t, root, "content/post/hero.png", "pngdata")

	if err := Build(config.DefaultOptions().
		WithConfig(filepath.Join(root, "shizuka.toml")).
		WithOutput(filepath.Join(root, "dist"))); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	html := readTestFile(t, root, "dist/post/index.html")
	if !strings.Contains(html, `src=/_assets/`) {
		t.Fatalf("expected fingerprinted asset URL in HTML, got %q", html)
	}
	if !strings.Contains(html, `<img src=/_assets/`) {
		t.Fatalf("expected asset helper output in template, got %q", html)
	}
	if !strings.Contains(html, `href=/about/`) {
		t.Fatalf("expected obsidian page link in HTML, got %q", html)
	}

	raw := readTestFile(t, root, "dist/post.md")
	if !strings.Contains(raw, "![Hero](hero.png)") || !strings.Contains(raw, "[[about|About page]]") {
		t.Fatalf("expected raw markdown variant, got %q", raw)
	}

	assetsDir := filepath.Join(root, "dist", "_assets")
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		t.Fatalf("read assets dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 fingerprinted asset, got %d", len(entries))
	}
}

func writeTestFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(strings.TrimPrefix(contents, "\n")), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func readTestFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}
