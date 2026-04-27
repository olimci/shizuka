package scaffold

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDecodeConfigFileRejectsExtraDocuments(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		body     string
		wantErr  string
	}{
		{
			name:     "yaml",
			filename: "shizuka.template.yaml",
			body:     "metadata:\n  slug: demo\n---\nmetadata:\n  slug: second\n",
			wantErr:  "unexpected extra YAML document",
		},
		{
			name:     "json",
			filename: "shizuka.template.json",
			body:     "{\"metadata\":{\"slug\":\"demo\"}}{}",
			wantErr:  "unexpected extra content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeConfigFile(tt.filename, strings.NewReader(tt.body), &TemplateCfg{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("decodeConfigFile() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFSAndBuildTemplate(t *testing.T) {
	fsys := fstest.MapFS{
		"shizuka.template.toml": &fstest.MapFile{Data: []byte(`
[metadata]
name = "Demo"
slug = "demo"

[files]
strip_suffixes = [".tmpl"]
templates = ["**/*.tmpl"]

[files.renames]
"_gitignore" = ".gitignore"
`)},
		"README.md.tmpl": &fstest.MapFile{Data: []byte("Hello {{.Name}}\n")},
		"_gitignore":     &fstest.MapFile{Data: []byte("dist/\n")},
		"nested/_env.tmpl": &fstest.MapFile{Data: []byte(
			"API_KEY={{.APIKey}}\n",
		)},
	}

	template, collection, err := LoadFS(context.Background(), fsys, ".")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if collection != nil {
		t.Fatalf("LoadFS() collection = %#v, want nil", collection)
	}

	target := t.TempDir()
	result, err := template.Build(target, BuildOptions{
		Variables: map[string]any{
			"Name":   "Shizuka",
			"APIKey": "secret",
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	files := append([]string(nil), result.FilesCreated...)
	slices.Sort(files)
	wantFiles := []string{".gitignore", "README.md", "nested/.env"}
	if !slices.Equal(files, wantFiles) {
		t.Fatalf("result.FilesCreated = %#v, want %#v", files, wantFiles)
	}

	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(readme) != "Hello Shizuka\n" {
		t.Fatalf("README.md = %q, want rendered template", readme)
	}

	env, err := os.ReadFile(filepath.Join(target, "nested", ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(env) != "API_KEY=secret\n" {
		t.Fatalf("nested/.env = %q, want rendered template", env)
	}
}

func TestLoadFSCollectionAndLookup(t *testing.T) {
	fsys := fstest.MapFS{
		"shizuka.collection.toml": &fstest.MapFile{Data: []byte(`
[templates]
items = ["blog"]
default = "blog"
`)},
		"blog/shizuka.template.toml": &fstest.MapFile{Data: []byte(`
[metadata]
name = "Blog"
slug = "blog"
`)},
	}

	template, collection, err := LoadFS(context.Background(), fsys, ".")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}
	if template != nil {
		t.Fatalf("LoadFS() template = %#v, want nil", template)
	}
	if collection == nil || collection.Get("blog") == nil {
		t.Fatalf("collection.Get(blog) = %#v, want template", collection)
	}
}

func TestLoadCollectionRejectsMismatchedSlug(t *testing.T) {
	src := newSource(fstest.MapFS{
		"shizuka.collection.toml": &fstest.MapFile{Data: []byte(`
[templates]
items = ["blog"]
`)},
		"blog/shizuka.template.toml": &fstest.MapFile{Data: []byte(`
[metadata]
slug = "news"
`)},
	}, ".", nil)

	_, err := LoadCollection(src, ".")
	if err == nil || !strings.Contains(err.Error(), "does not match directory name") {
		t.Fatalf("LoadCollection() error = %v, want slug mismatch error", err)
	}
}

func TestHelpers(t *testing.T) {
	if !matchesGlobs("content/post.md.tmpl", []string{"**/*.tmpl"}) {
		t.Fatal("matchesGlobs() = false, want true")
	}
	if isScaffoldConfigFile("shizuka.template.toml") != true {
		t.Fatal("isScaffoldConfigFile(template) = false, want true")
	}
	if got := (&Template{
		Config: TemplateCfg{
			Files: TemplateCfgFiles{
				StripSuffixes: []string{".tmpl"},
				Renames: map[string]string{
					"_gitignore": ".gitignore",
				},
			},
		},
	}).transformPath("nested/_env.tmpl"); got != "nested/.env" {
		t.Fatalf("transformPath() = %q, want %q", got, "nested/.env")
	}
	if got, err := relSourcePath("content", "content/posts/hello.md"); err != nil || got != "posts/hello.md" {
		t.Fatalf("relSourcePath() = %q, %v; want %q, nil", got, err, "posts/hello.md")
	}
	if _, err := relSourcePath("content", "templates/page.tmpl"); err == nil {
		t.Fatal("relSourcePath() error = nil, want error")
	}

	_, ok := any(fstest.MapFS{}).(fs.FS)
	if !ok {
		t.Fatal("fstest.MapFS does not implement fs.FS")
	}
}
