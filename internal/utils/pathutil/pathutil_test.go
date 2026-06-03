package pathutil

import (
	"path/filepath"
	"testing"
)

func TestCleanContentPath(t *testing.T) {
	t.Run("accepts clean relative paths", func(t *testing.T) {
		got, err := CleanContentPath("posts/hello.md")
		if err != nil {
			t.Fatal(err)
		}
		if got != "posts/hello.md" {
			t.Fatalf("path = %q, want posts/hello.md", got)
		}
	})

	t.Run("rejects unclean paths", func(t *testing.T) {
		if _, err := CleanContentPath("posts/../hello.md"); err == nil {
			t.Fatal("expected unclean path error")
		}
	})

	t.Run("rejects paths escaping root", func(t *testing.T) {
		if _, err := CleanContentPath("../hello.md"); err == nil {
			t.Fatal("expected escaping path error")
		}
	})
}

func TestResolvePathCleansRootAndJoinsRelativePath(t *testing.T) {
	got, err := ResolvePath("site/.", "content/page.md")
	if err != nil {
		t.Fatal(err)
	}

	if got.Root != "site" {
		t.Fatalf("root = %q, want site", got.Root)
	}
	want := filepath.Clean(filepath.Join("site", "content", "page.md"))
	if got.Path != want {
		t.Fatalf("path = %q, want %q", got.Path, want)
	}
}

func TestRoutePathForContentPath(t *testing.T) {
	tests := map[string]string{
		"index.md":                 "/",
		"posts/index.md":           "/posts/",
		"posts/Hello World!.md":    "/posts/hello-world/",
		"guides/API_Reference.md":  "/guides/API_Reference/",
		"notes/2025.01.02.md":      "/notes/2025.01.02/",
		"nested/Already-Safe.html": "/nested/Already-Safe/",
	}

	for input, want := range tests {
		got, err := RoutePathForContentPath(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if got != want {
			t.Fatalf("%s: route = %q, want %q", input, got, want)
		}
	}
}

func TestValidateRoutePath(t *testing.T) {
	valid := []string{"/", "/posts/", "/posts/API_Reference-1.0~/"}
	for _, raw := range valid {
		got, err := ValidateRoutePath(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got != raw {
			t.Fatalf("route = %q, want %q", got, raw)
		}
	}

	invalid := []string{"", "posts/", "/posts", "/posts/../", "/has space/", "/query?x/"}
	for _, raw := range invalid {
		if _, err := ValidateRoutePath(raw); err == nil {
			t.Fatalf("%s: expected validation error", raw)
		}
	}
}

func TestCanonicalAndOutputPaths(t *testing.T) {
	canon, err := CanonicalPageURL("https://example.com/blog", "/posts/hello/")
	if err != nil {
		t.Fatal(err)
	}
	if canon != "https://example.com/blog/posts/hello/" {
		t.Fatalf("canon = %q, want https://example.com/blog/posts/hello/", canon)
	}

	if got := OutputPathForRoutePath("/posts/hello/"); got != "posts/hello/index.html" {
		t.Fatalf("output = %q, want posts/hello/index.html", got)
	}
	if got := OutputPathForRoutePath("/"); got != "index.html" {
		t.Fatalf("output = %q, want index.html", got)
	}
}
