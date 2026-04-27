package build

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
)

func TestBuildErrorAndFailureHelpers(t *testing.T) {
	claim := manifest.Claim{Source: "content/post.md"}
	base := errors.New("boom")
	wrapped := WrapError(claim, base)

	if wrapped.Source() != "content/post.md" || wrapped.Location() != "content/post.md" {
		t.Fatalf("wrapped location helpers = %#v, want source location", wrapped)
	}
	if wrapped.Description() != "boom" {
		t.Fatalf("wrapped.Description() = %q, want %q", wrapped.Description(), "boom")
	}

	rewrapped := WrapError(manifest.Claim{Target: "dist/post/index.html"}, wrapped)
	if rewrapped != wrapped {
		t.Fatal("WrapError(non-zero existing claim) should preserve original BuildError")
	}

	zeroClaimWrapped := WrapError(manifest.Claim{}, base)
	reclaimed := WrapError(claim, zeroClaimWrapped)
	if reclaimed.Source() != "content/post.md" {
		t.Fatalf("reclaimed.Source() = %q, want updated claim source", reclaimed.Source())
	}

	state := &errorState{}
	state.Add(claim, base)
	if !state.HasErrors() || len(state.Slice()) != 1 {
		t.Fatalf("errorState = %#v, want one stored error", state.Slice())
	}

	failure := &Failure{Errors: []*BuildError{wrapped}}
	if failure.Count() != 1 || !failure.HasErrors() {
		t.Fatalf("failure helpers = %#v, want count 1", failure)
	}
	if failure.Summary() != "1 error" {
		t.Fatalf("failure.Summary() = %q, want %q", failure.Summary(), "1 error")
	}
	if got, ok := AsFailure(failure); !ok || got != failure {
		t.Fatalf("AsFailure() = %#v, %v; want same failure", got, ok)
	}
}

func TestContentHelpers(t *testing.T) {
	if !isPageSourceExt(".md") || isPageSourceExt(".png") {
		t.Fatal("isPageSourceExt() returned unexpected result")
	}

	assets := map[string]*transforms.PageAsset{
		"images/logo.png": {URL: "/assets/logo.png"},
	}
	body := `<img src="images/logo.png"><a href="./images/logo.png">x</a><a href="https://example.com">y</a>`
	rewritten := rewriteHTMLBundleLinks(body, assets)
	if rewritten == body {
		t.Fatalf("rewriteHTMLBundleLinks() did not rewrite body: %q", rewritten)
	}
	if got := resolveBundleURL("../logo.png", assets); got != "" {
		t.Fatalf("resolveBundleURL(escape) = %q, want empty", got)
	}
	if got := resolveBundleURL("images/logo.png", assets); got != "/assets/logo.png" {
		t.Fatalf("resolveBundleURL() = %q, want %q", got, "/assets/logo.png")
	}

	filesByDir, descendantsByDir := indexContentFiles([]string{
		"posts/hello.md",
		"posts/hello.png",
		"posts/hello/banner.jpg",
		"posts/other.txt",
	})
	page := &transforms.Page{SourcePath: "content/posts/hello.md", ContentPath: "posts/hello.md"}
	sources, err := pageBundleSources(page, filesByDir, descendantsByDir)
	if err != nil {
		t.Fatalf("pageBundleSources() error = %v", err)
	}
	if !slices.Equal(sources, []string{"posts/hello.png", "posts/hello/banner.jpg"}) {
		t.Fatalf("pageBundleSources() = %#v, want direct and nested bundle assets", sources)
	}

	_, err = pageBundleSources(page, filesByDir, map[string][]string{
		"posts/hello": {"posts/hello/index.md"},
	})
	if err == nil {
		t.Fatal("pageBundleSources(bundle page source) error = nil, want error")
	}
}

func TestBuildPageAssetAndAttachPageFileMeta(t *testing.T) {
	root := t.TempDir()
	contentRoot := filepath.Join(root, "content")
	if err := os.MkdirAll(filepath.Join(contentRoot, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(contentRoot, "posts", "hello.png")
	if err := os.WriteFile(source, []byte("image-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	sc := &StepContext{SourceRoot: root}
	page := &transforms.Page{
		SourcePath:  "content/posts/hello.md",
		ContentPath: "posts/hello.md",
		URLPath:     "posts/hello",
		Assets:      map[string]*transforms.PageAsset{},
	}

	asset, err := buildPageAsset(sc, page, "hello.png", "content/posts/hello.png", "adjacent", "_assets")
	if err != nil {
		t.Fatalf("buildPageAsset(adjacent) error = %v", err)
	}
	if asset.URL != "/posts/hello/hello.png" || asset.Standalone {
		t.Fatalf("adjacent asset = %#v, want page-relative URL and non-standalone", asset)
	}

	fingerprinted, err := buildPageAsset(sc, page, "hello.png", "content/posts/hello.png", "fingerprinted", "_assets")
	if err != nil {
		t.Fatalf("buildPageAsset(fingerprinted) error = %v", err)
	}
	if filepath.Ext(fingerprinted.Target) != ".png" || fingerprinted.URL[:8] != "/_assets" || !fingerprinted.Standalone {
		t.Fatalf("fingerprinted asset = %#v, want assets dir target and standalone", fingerprinted)
	}

	attachPageFileMeta(page, source)
	if !page.File.Available || page.File.Size != int64(len("image-data")) {
		t.Fatalf("page.File = %#v, want populated file metadata", page.File)
	}
	if page.Created.IsZero() || page.Updated.IsZero() || page.PubDate.IsZero() {
		t.Fatalf("page timestamps = created:%v updated:%v pub:%v; want non-zero", page.Created, page.Updated, page.PubDate)
	}
}

func TestCacheAndCloningHelpers(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fp := fileFingerprint{Created: now, Updated: now, Size: 12}
	if !fp.Equal(fileFingerprint{Created: now, Updated: now, Size: 12}) {
		t.Fatal("fileFingerprint.Equal() = false, want true")
	}
	if sameBundleInputs(map[string]fileFingerprint{"a": fp}, map[string]fileFingerprint{"a": fp}) != true {
		t.Fatal("sameBundleInputs() = false, want true")
	}

	if normalized := normalizeChangedPaths(nil); normalized != nil {
		t.Fatalf("normalizeChangedPaths(nil) = %#v, want nil", normalized)
	}

	normalized := normalizeChangedPaths([]string{" ./b ", "./a", "", "./a"})
	if len(normalized) != 2 || normalized[0] >= normalized[1] {
		t.Fatalf("normalizeChangedPaths() = %#v, want sorted unique values", normalized)
	}

	page := &transforms.Page{
		Title:   "Hello",
		Aliases: []string{"/hello"},
		Tags:    []string{"go"},
		Params:  map[string]any{"author": "oliver"},
		Assets: map[string]*transforms.PageAsset{
			"logo.png": {Key: "logo.png"},
		},
		Queries: map[string]*transforms.QueryResult{
			"recent": {
				Rows:  []map[string]any{{"title": "Hello"}},
				Pages: []*transforms.Page{{Title: "Nested"}},
			},
		},
		Headers: map[string]string{"X-Test": "yes"},
		Pagination: &transforms.PaginationState{
			QueryKey: "recent",
		},
		Group: &transforms.QueryGroupState{
			QueryKey: "recent",
			Value:    "go",
		},
	}
	cloned := cloneCachedPage(page)
	if cloned == page || cloned.Assets["logo.png"].Owner != cloned {
		t.Fatalf("cloneCachedPage() = %#v, want deep clone with asset owner rebound", cloned)
	}
	page.Aliases[0] = "/changed"
	page.Queries["recent"].Rows[0]["title"] = "Changed"
	if cloned.Aliases[0] != "/hello" || cloned.Queries["recent"].Rows[0]["title"] != "Hello" {
		t.Fatalf("cloneCachedPage() did not isolate mutable fields: %#v", cloned)
	}

	indexClone := clonePageForIndexCache(page)
	if len(indexClone.Assets) != 0 || indexClone.Body != "" || indexClone.Queries != nil || indexClone.Pagination != nil || indexClone.Group != nil {
		t.Fatalf("clonePageForIndexCache() = %#v, want stripped heavy fields", indexClone)
	}
}

func TestApplyGitMetadataBackfillsWhenEnabled(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	page := &transforms.Page{}
	page.Source.HasCreated = false
	page.Source.HasUpdated = false

	applyGitMetadata(page, &transforms.PageGitMeta{
		Tracked: true,
		Created: created,
		Updated: updated,
	}, true)
	if page.Created != created || page.Updated != updated || page.PubDate != updated {
		t.Fatalf("applyGitMetadata(backfill) page = %#v, want created/updated/pubdate backfilled", page)
	}

	page = &transforms.Page{Created: created}
	page.Source.HasCreated = true
	applyGitMetadata(page, &transforms.PageGitMeta{Created: updated, Updated: updated}, true)
	if page.Created != created {
		t.Fatalf("applyGitMetadata(existing created) = %v, want preserved %v", page.Created, created)
	}

	page = &transforms.Page{}
	applyGitMetadata(page, &transforms.PageGitMeta{Created: created, Updated: updated}, false)
	if !page.Created.IsZero() || !page.Updated.IsZero() {
		t.Fatalf("applyGitMetadata(no backfill) page = %#v, want no created/updated mutation", page)
	}
}
