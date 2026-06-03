package transforms

import (
	"errors"
	"html/template"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/olimci/shizuka/internal/frontmatter"
)

func TestBuildPageFromMarkdown(t *testing.T) {
	page, err := BuildPage(fstest.MapFS{
		"posts/hello.md": &fstest.MapFile{
			Data: []byte("+++\ntitle = \"Hello\"\ntags = [\"go\"]\n+++\n# Body"),
		},
	}, "posts/hello.md", "pages", frontmatter.Defaults{Template: "page"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if page.SourcePath != "posts/hello.md" {
		t.Fatalf("source path = %q, want posts/hello.md", page.SourcePath)
	}
	if page.Title != "Hello" || page.Template != "page" {
		t.Fatalf("page = %#v, want title and default template", page)
	}
	if page.Preprocess != "markdown" {
		t.Fatalf("preprocess = %q, want markdown", page.Preprocess)
	}
	if page.RawBody != "# Body" {
		t.Fatalf("raw body = %q, want # Body", page.RawBody)
	}
	if page.Body != "" {
		t.Fatalf("body = %q, want empty until markdown preprocessing", page.Body)
	}
}

func TestBuildPageFromHTML(t *testing.T) {
	page, err := BuildPage(fstest.MapFS{
		"about.html": &fstest.MapFile{
			Data: []byte("---\ntitle: About\n---\n<p>About</p>"),
		},
	}, "about.html", "pages", frontmatter.Defaults{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if page.Title != "About" {
		t.Fatalf("title = %q, want About", page.Title)
	}
	if page.Preprocess != "" {
		t.Fatalf("preprocess = %q, want empty", page.Preprocess)
	}
	if page.Body != template.HTML("<p>About</p>") {
		t.Fatalf("body = %q, want raw HTML body", page.Body)
	}
}

func TestBuildPageFromDataFile(t *testing.T) {
	page, err := BuildPage(fstest.MapFS{
		"note.jsonc": &fstest.MapFile{
			Data: []byte(`{
				"title": "Note",
				"section": "notes",
				"body": "**Body**",
				"body_markdown": true
			}`),
		},
	}, "note.jsonc", "pages", frontmatter.Defaults{}, map[string]frontmatter.Defaults{
		"notes": {Template: "note"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if page.Title != "Note" || page.Section != "notes" || page.Template != "note" {
		t.Fatalf("page = %#v, want data fields and section default", page)
	}
	if page.RawBody != "**Body**" {
		t.Fatalf("raw body = %q, want markdown body", page.RawBody)
	}
	if page.Preprocess != "markdown" {
		t.Fatalf("preprocess = %q, want markdown", page.Preprocess)
	}
}

func TestBuildPageRejectsUnsupportedContentType(t *testing.T) {
	_, err := BuildPage(fstest.MapFS{
		"image.png": &fstest.MapFile{Data: []byte("png")},
	}, "image.png", "pages", frontmatter.Defaults{}, nil)

	if !errors.Is(err, ErrUnsupportedContentType) {
		t.Fatalf("err = %v, want ErrUnsupportedContentType", err)
	}
}

func TestBuildPageReturnsReadError(t *testing.T) {
	_, err := BuildPage(fstest.MapFS{}, "missing.md", "pages", frontmatter.Defaults{}, nil)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want fs.ErrNotExist", err)
	}
}
