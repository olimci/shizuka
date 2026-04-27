package pathutil

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanContentPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "root index", input: "index.md", want: "index.md"},
		{name: "cleans path", input: "posts/../hello.md", want: "hello.md"},
		{name: "dot", input: ".", want: "."},
		{name: "rejects absolute", input: "/tmp/file.md", wantErr: "absolute paths"},
		{name: "rejects escape", input: "../file.md", wantErr: "escapes source root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CleanContentPath(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("CleanContentPath(%q) error = %v, want substring %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CleanContentPath(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("CleanContentPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanSlugAndURLPathHelpers(t *testing.T) {
	got, err := CleanSlug("/blog/hello-world/")
	if err != nil {
		t.Fatalf("CleanSlug() error = %v", err)
	}
	if got != "blog/hello-world" {
		t.Fatalf("CleanSlug() = %q, want %q", got, "blog/hello-world")
	}

	if _, err := CleanSlug("blog/hello world"); err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Fatalf("CleanSlug() error = %v, want whitespace validation error", err)
	}

	if got := URLPathForContentPath("Posts/Hello World.md"); got != "Posts/hello-world" {
		t.Fatalf("URLPathForContentPath() = %q, want %q", got, "Posts/hello-world")
	}
	if got := URLPathForContentPath("notes/index.md"); got != "notes" {
		t.Fatalf("URLPathForContentPath(index) = %q, want %q", got, "notes")
	}
}

func TestCanonicalPageURLAndLeadingSlash(t *testing.T) {
	got, err := CanonicalPageURL("https://example.com/base", "posts/hello")
	if err != nil {
		t.Fatalf("CanonicalPageURL() error = %v", err)
	}
	if got != "https://example.com/base/posts/hello/" {
		t.Fatalf("CanonicalPageURL() = %q, want %q", got, "https://example.com/base/posts/hello/")
	}

	if got := EnsureLeadingSlash("posts/hello"); got != "/posts/hello" {
		t.Fatalf("EnsureLeadingSlash() = %q, want %q", got, "/posts/hello")
	}
	if got := EnsureLeadingSlash("https://example.com"); got != "https://example.com" {
		t.Fatalf("EnsureLeadingSlash(external) = %q, want unchanged", got)
	}
}

func TestRelPathWithinAndEscapesRoot(t *testing.T) {
	if got, err := RelPathWithin("content", "content/posts/hello.md"); err != nil || got != "posts/hello.md" {
		t.Fatalf("RelPathWithin() = %q, %v; want %q, nil", got, err, "posts/hello.md")
	}

	_, err := RelPathWithin("content", "templates/page.tmpl")
	if err == nil {
		t.Fatal("RelPathWithin() error = nil, want error")
	}
	if !errors.Is(err, err) || !strings.Contains(err.Error(), "not within") {
		t.Fatalf("RelPathWithin() error = %v, want not within error", err)
	}

	if !EscapesRoot("../posts") {
		t.Fatal("EscapesRoot(\"../posts\") = false, want true")
	}
	if EscapesRoot("posts/hello") {
		t.Fatal("EscapesRoot(\"posts/hello\") = true, want false")
	}
}
