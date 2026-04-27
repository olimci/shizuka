package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCandidatesPrioritizeProvidedExtension(t *testing.T) {
	got := configCandidates("nested/shizuka.yaml")
	want := []string{
		filepath.Clean("nested/shizuka.yaml"),
		filepath.Clean("nested/shizuka.toml"),
		filepath.Clean("nested/shizuka.yml"),
		filepath.Clean("nested/shizuka.json"),
	}

	if len(got) != len(want) {
		t.Fatalf("len(configCandidates) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("configCandidates[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadResolvesExtensionlessPathAndAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "shizuka.yaml")

	content := `
site:
  url: https://example.com
headers: {}
redirects: {}
rss: {}
sitemap: {}
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(strings.TrimSuffix(configPath, ".yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Root != dir {
		t.Fatalf("cfg.Root = %q, want %q", cfg.Root, dir)
	}
	if cfg.Headers == nil || cfg.Headers.Output != "_headers" {
		t.Fatalf("cfg.Headers.Output = %v, want %q", cfg.Headers, "_headers")
	}
	if cfg.Redirects == nil || cfg.Redirects.Output != "_redirects" || cfg.Redirects.Shorten != "/s" {
		t.Fatalf("cfg.Redirects = %#v, want default output/shorten", cfg.Redirects)
	}
	if cfg.RSS == nil || cfg.RSS.Output != "rss.xml" {
		t.Fatalf("cfg.RSS.Output = %v, want %q", cfg.RSS, "rss.xml")
	}
	if cfg.Sitemap == nil || cfg.Sitemap.Output != "sitemap.xml" {
		t.Fatalf("cfg.Sitemap.Output = %v, want %q", cfg.Sitemap, "sitemap.xml")
	}
	if cfg.Site.Params == nil {
		t.Fatal("cfg.Site.Params is nil")
	}
	if cfg.Content.Defaults.Params == nil {
		t.Fatal("cfg.Content.Defaults.Params is nil")
	}

	paths, globs, err := cfg.WatchedPaths()
	if err != nil {
		t.Fatalf("WatchedPaths() error = %v", err)
	}

	wantPaths := []string{
		filepath.Join(dir, "static"),
		filepath.Join(dir, "content"),
	}
	for i := range wantPaths {
		if paths[i] != wantPaths[i] {
			t.Fatalf("paths[%d] = %q, want %q", i, paths[i], wantPaths[i])
		}
	}
	if len(globs) != 1 || globs[0] != filepath.Join(dir, "templates", "*.tmpl") {
		t.Fatalf("globs = %#v, want %q", globs, filepath.Join(dir, "templates", "*.tmpl"))
	}
}

func TestValidateRejectsInvalidBundleMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Site.URL = "https://example.com"
	cfg.Content.Bundles.Mode = "bad"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "content.bundles.mode") {
		t.Fatalf("Validate() error = %v, want bundle mode validation error", err)
	}
}

func TestDecodeConfigBytesRejectsUnsupportedExtension(t *testing.T) {
	err := decodeConfigBytes("shizuka.txt", []byte("site.url = 'https://example.com'"), &Config{})
	if err == nil || !strings.Contains(err.Error(), "unsupported config file type") {
		t.Fatalf("decodeConfigBytes() error = %v, want unsupported type error", err)
	}
}

func TestConfigGoldmarkBuildAppliesExtensions(t *testing.T) {
	cfg := ConfigGoldmark{
		Extensions: []string{"table", "footnotes"},
		Parser: ConfigGoldmarkParser{
			AutoHeadingID: true,
		},
		Renderer: ConfigGoldmarkRenderer{
			Hardbreaks: true,
		},
	}

	var out bytes.Buffer
	source := "| a |\n| - |\n| b |\n\nline one\nline two\n"
	if err := cfg.Build().Convert([]byte(source), &out); err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, "<table>") {
		t.Fatalf("rendered markdown missing table: %q", rendered)
	}
	if !strings.Contains(rendered, "line one<br>") {
		t.Fatalf("rendered markdown missing hard line break: %q", rendered)
	}
}
