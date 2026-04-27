package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchPatternAndRedirect(t *testing.T) {
	if matched, splat := matchPattern("/blog/*", "/blog/hello"); !matched || splat != "hello" {
		t.Fatalf("matchPattern() = %v, %q; want true, hello", matched, splat)
	}
	if matched, _ := matchPattern("/blog/*", "/docs/hello"); matched {
		t.Fatal("matchPattern(non-match) = true, want false")
	}

	action, ok := matchRedirect("/old/path", []redirectRule{{from: "/old/*", to: "new/:splat", status: 200}})
	if !ok || action.kind != redirectActionRewrite || action.target != "/new/path" {
		t.Fatalf("matchRedirect(rewrite) = %#v, %v; want rewrite /new/path", action, ok)
	}

	action, ok = matchRedirect("/ext", []redirectRule{{from: "/ext", to: "https://example.com", status: 200}})
	if !ok || action.kind != redirectActionRewrite || action.target != "https://example.com" {
		t.Fatalf("matchRedirect(external) = %#v, %v; want rewrite with external target", action, ok)
	}

	action, ok = matchRedirect("/gone", []redirectRule{{from: "/gone", to: "/elsewhere", status: 410}})
	if !ok || action.kind != redirectActionStatus || action.status != 410 {
		t.Fatalf("matchRedirect(status) = %#v, %v; want status 410", action, ok)
	}
}

func TestParseHeadersAndRedirectsFiles(t *testing.T) {
	dir := t.TempDir()

	headersPath := filepath.Join(dir, "_headers")
	if err := os.WriteFile(headersPath, []byte(`
# comment
/docs/*
  X-Test: enabled
  Cache-Control: no-store
`), 0o644); err != nil {
		t.Fatal(err)
	}

	headers, err := parseHeadersFile(headersPath)
	if err != nil {
		t.Fatalf("parseHeadersFile() error = %v", err)
	}
	if len(headers) != 1 || headers[0].pattern != "/docs/*" || headers[0].headers["X-Test"] != "enabled" {
		t.Fatalf("headers = %#v, want one parsed rule", headers)
	}

	redirectsPath := filepath.Join(dir, "_redirects")
	if err := os.WriteFile(redirectsPath, []byte(`
https://example.com/from /to 301
/rewrite /target 200 # inline comment
`), 0o644); err != nil {
		t.Fatal(err)
	}

	redirects, err := parseRedirectsFile(redirectsPath)
	if err != nil {
		t.Fatalf("parseRedirectsFile() error = %v", err)
	}
	if len(redirects) != 2 {
		t.Fatalf("len(redirects) = %d, want 2", len(redirects))
	}
	if redirects[0].from != "/from" || redirects[0].status != 301 {
		t.Fatalf("redirects[0] = %#v, want normalized source and status", redirects[0])
	}
	if redirects[1].from != "/rewrite" || redirects[1].to != "/target" || redirects[1].status != 200 {
		t.Fatalf("redirects[1] = %#v, want rewrite rule", redirects[1])
	}
}

func TestResolvePathAndInternalControlPaths(t *testing.T) {
	dist := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dist, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "docs", "index.html"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "asset.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := NewStaticHandler(dist, StaticHandlerOptions{}).(*StaticHandler)

	if filePath, redirectPath, ok := handler.resolvePath("/docs"); !ok || redirectPath != "/docs/" || filePath != "" {
		t.Fatalf("resolvePath(/docs) = %q, %q, %v; want empty, /docs/, true", filePath, redirectPath, ok)
	}
	if filePath, redirectPath, ok := handler.resolvePath("/docs/"); !ok || redirectPath != "" || filepath.Base(filePath) != "index.html" {
		t.Fatalf("resolvePath(/docs/) = %q, %q, %v; want index.html, empty, true", filePath, redirectPath, ok)
	}
	if filePath, redirectPath, ok := handler.resolvePath("/asset.txt"); !ok || redirectPath != "" || filepath.Base(filePath) != "asset.txt" {
		t.Fatalf("resolvePath(/asset.txt) = %q, %q, %v; want asset.txt, empty, true", filePath, redirectPath, ok)
	}
	if !handler.isInternalControlPath("/_headers") || !handler.isInternalControlPath("/_redirects") {
		t.Fatal("isInternalControlPath() = false, want true for control files")
	}
}
