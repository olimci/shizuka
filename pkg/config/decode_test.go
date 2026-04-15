package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadFSSupportsTOMLYAMLAndJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
	}{
		{
			name: "toml",
			path: "shizuka.toml",
			body: `
version = "1.2.3"

[site]
title = "TOML"
description = "desc"
url = "https://example.com/"

[site.queries.recent]
query = "select 1"

[content.defaults]
template = "page"
`,
		},
		{
			name: "yaml",
			path: "shizuka.yaml",
			body: `
version: "1.2.3"
site:
  title: YAML
  description: desc
  url: https://example.com/
  queries:
    recent:
      query: select 1
content:
  defaults:
    template: page
`,
		},
		{
			name: "json",
			path: "shizuka.json",
			body: `{
  "version": "1.2.3",
  "site": {
    "title": "JSON",
    "description": "desc",
    "url": "https://example.com/",
    "queries": {
      "recent": {
        "query": "select 1"
      }
    }
  },
  "content": {
    "defaults": {
      "template": "page"
    }
  }
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := LoadFS(fstest.MapFS{
				tt.path: &fstest.MapFile{Data: []byte(tt.body)},
			}, tt.path)
			if err != nil {
				t.Fatalf("LoadFS() error = %v", err)
			}

			if cfg.Version != "1.2.3" {
				t.Fatalf("expected version to be decoded, got %q", cfg.Version)
			}
			if cfg.Site.Title == "" {
				t.Fatalf("expected site title to be decoded")
			}
			if cfg.Site.URL != "https://example.com/" {
				t.Fatalf("expected site url to be decoded, got %q", cfg.Site.URL)
			}
			if cfg.Site.Queries["recent"].Query != "select 1" {
				t.Fatalf("expected site query to be decoded, got %#v", cfg.Site.Queries["recent"])
			}
			if cfg.Content.Defaults.Template != "page" {
				t.Fatalf("expected content default template to be decoded, got %q", cfg.Content.Defaults.Template)
			}
		})
	}
}

func TestLoadFSRejectsOldBuildStepsSchema(t *testing.T) {
	t.Parallel()

	_, err := LoadFS(fstest.MapFS{
		"shizuka.toml": &fstest.MapFile{Data: []byte(`
[site]
url = "https://example.com/"

[build.steps.content]
source = "content"
`)},
	}, "shizuka.toml")
	if err == nil {
		t.Fatal("expected old build.steps schema to be rejected")
	}
	if !strings.Contains(err.Error(), "build.steps") {
		t.Fatalf("expected error to mention build.steps, got %v", err)
	}
}

func TestResolvePathFallsBackAcrossSupportedExtensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "shizuka.yaml")
	if err := os.WriteFile(yamlPath, []byte("site:\n  url: https://example.com/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resolved, err := ResolvePath(filepath.Join(dir, "shizuka.toml"))
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if resolved != yamlPath {
		t.Fatalf("expected ResolvePath() = %q, got %q", yamlPath, resolved)
	}
}
