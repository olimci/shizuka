package build

import (
	"html/template"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/transforms"
)

func TestQueryResultOrdersPagesByTime(t *testing.T) {
	t.Parallel()

	older := &transforms.Page{
		Title:       "Older",
		SourcePath:  "content/posts/older.md",
		ContentPath: "posts/older.md",
		URLPath:     "posts/older",
		OutputPath:  "posts/older/index.html",
		Template:    "post",
		Section:     "posts",
		Created:     time.Date(2026, 1, 14, 17, 3, 27, 0, time.UTC),
	}
	newer := &transforms.Page{
		Title:       "Newer",
		SourcePath:  "content/posts/newer.md",
		ContentPath: "posts/newer.md",
		URLPath:     "posts/newer",
		OutputPath:  "posts/newer/index.html",
		Template:    "post",
		Section:     "posts",
		Created:     time.Date(2026, 4, 14, 18, 48, 24, 0, time.UTC),
		Updated:     time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
	}

	db, err := BuildQueryDB([]*transforms.Page{older, newer})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	byDate, err := queryResult(db, "select * from pages order by Created desc")
	if err != nil {
		t.Fatalf("query by date failed: %v", err)
	}
	if len(byDate.Pages) != 2 || byDate.Pages[0] != newer || byDate.Pages[1] != older {
		t.Fatalf("unexpected page order by created time: %#v", byDate.Pages)
	}

	byUpdated, err := queryResult(db, "select * from pages order by Updated asc")
	if err != nil {
		t.Fatalf("query by updated failed: %v", err)
	}
	if len(byUpdated.Rows) != 2 {
		t.Fatalf("unexpected row count by updated: %d", len(byUpdated.Rows))
	}
	if got := byUpdated.Rows[0]["Title"]; got != "Older" {
		t.Fatalf("expected zero-updated page first, got %#v", got)
	}
	if got := byUpdated.Rows[1]["Title"]; got != "Newer" {
		t.Fatalf("expected newer page second, got %#v", got)
	}
}

func TestComputePageQueriesGroupedWithPagination(t *testing.T) {
	t.Parallel()

	root := &transforms.Page{
		Title:       "Tags",
		SourcePath:  "content/tags.md",
		ContentPath: "tags.md",
		URLPath:     "tags",
		OutputPath:  "tags/index.html",
		Template:    "page",
		Source: transforms.PageSource{
			FrontmatterDoc: &transforms.FrontmatterDoc{
				Meta: transforms.Frontmatter{
					Queries: map[string]transforms.PageQueryDef{
						"tagged": {
							Query:           "select _page, tag.value as tag, Title from pages p join unnest(p.Tags) tag on true where p.Section = 'posts' order by tag.value asc, Title asc",
							GroupBy:         "tag",
							GroupFormat:     "%s",
							Paginate:        true,
							PageSize:        1,
							PageFormat:      "%02d",
							RedirectPageOne: true,
							Template:        "list",
						},
					},
				},
			},
		},
	}
	alphaOne := &transforms.Page{
		Title:       "Alpha One",
		SourcePath:  "content/posts/alpha-one.md",
		ContentPath: "posts/alpha-one.md",
		URLPath:     "posts/alpha-one",
		OutputPath:  "posts/alpha-one/index.html",
		Template:    "post",
		Section:     "posts",
		Tags:        []string{"alpha", "beta"},
	}
	alphaTwo := &transforms.Page{
		Title:       "Alpha Two",
		SourcePath:  "content/posts/alpha-two.md",
		ContentPath: "posts/alpha-two.md",
		URLPath:     "posts/alpha-two",
		OutputPath:  "posts/alpha-two/index.html",
		Template:    "post",
		Section:     "posts",
		Tags:        []string{"alpha"},
	}
	site := &transforms.Site{URL: "https://example.com/"}

	db, err := BuildQueryDB([]*transforms.Page{root, alphaOne, alphaTwo})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}
	tmpl := template.Must(template.New("").Parse(`{{ define "page" }}page{{ end }}{{ define "list" }}list{{ end }}`))

	pages, err := ComputePageQueries(site, []*transforms.Page{root, alphaOne, alphaTwo}, tmpl, db)
	if err != nil {
		t.Fatalf("ComputePageQueries failed: %v", err)
	}

	byURL := make(map[string]*transforms.Page)
	for _, page := range pages {
		byURL[page.URLPath] = page
	}

	alphaPage1 := byURL["tags/alpha/01"]
	if alphaPage1 == nil {
		t.Fatalf("expected tags/alpha/01 page, got %#v", byURL)
	}
	if alphaPage1.Group == nil || alphaPage1.Group.Value != "alpha" {
		t.Fatalf("unexpected alpha group state: %#v", alphaPage1.Group)
	}
	if len(alphaPage1.Aliases) != 1 || alphaPage1.Aliases[0] != "tags/alpha" {
		t.Fatalf("unexpected alpha aliases: %#v", alphaPage1.Aliases)
	}
	if alphaPage1.Pagination == nil || alphaPage1.Pagination.TotalPages != 2 || alphaPage1.Pagination.NextURL != "/tags/alpha/02" {
		t.Fatalf("unexpected alpha pagination: %#v", alphaPage1.Pagination)
	}
	if got := alphaPage1.Queries["tagged"].Pages[0]; got != alphaOne {
		t.Fatalf("unexpected alpha page 1 result: %#v", got)
	}

	alphaPage2 := byURL["tags/alpha/02"]
	if alphaPage2 == nil {
		t.Fatalf("expected tags/alpha/02 page")
	}
	if alphaPage2.Pagination == nil || alphaPage2.Pagination.PrevURL != "/tags/alpha/01" {
		t.Fatalf("unexpected alpha page 2 pagination: %#v", alphaPage2.Pagination)
	}
	if got := alphaPage2.Queries["tagged"].Pages[0]; got != alphaTwo {
		t.Fatalf("unexpected alpha page 2 result: %#v", got)
	}

	betaPage1 := byURL["tags/beta/01"]
	if betaPage1 == nil {
		t.Fatalf("expected tags/beta/01 page")
	}
	if betaPage1.Group == nil || betaPage1.Group.Value != "beta" {
		t.Fatalf("unexpected beta group state: %#v", betaPage1.Group)
	}
	if got := betaPage1.Queries["tagged"].Pages[0]; got != alphaOne {
		t.Fatalf("unexpected beta result: %#v", got)
	}
}

func TestComputePageQueriesRedirectPageOne(t *testing.T) {
	t.Parallel()

	listing := &transforms.Page{
		Title:       "Posts",
		SourcePath:  "content/posts.md",
		ContentPath: "posts.md",
		URLPath:     "posts",
		OutputPath:  "posts/index.html",
		Template:    "page",
		Source: transforms.PageSource{
			FrontmatterDoc: &transforms.FrontmatterDoc{
				Meta: transforms.Frontmatter{
					Queries: map[string]transforms.PageQueryDef{
						"posts": {
							Query:           "select * from pages where Section = 'posts' order by Title asc",
							Paginate:        true,
							PageSize:        1,
							PageFormat:      "%02d",
							RedirectPageOne: true,
							Template:        "list",
						},
					},
				},
			},
		},
	}
	postOne := &transforms.Page{
		Title:       "Alpha",
		SourcePath:  "content/posts/alpha.md",
		ContentPath: "posts/alpha.md",
		URLPath:     "posts/alpha",
		OutputPath:  "posts/alpha/index.html",
		Template:    "post",
		Section:     "posts",
	}
	postTwo := &transforms.Page{
		Title:       "Beta",
		SourcePath:  "content/posts/beta.md",
		ContentPath: "posts/beta.md",
		URLPath:     "posts/beta",
		OutputPath:  "posts/beta/index.html",
		Template:    "post",
		Section:     "posts",
	}
	site := &transforms.Site{URL: "https://example.com/"}
	db, err := BuildQueryDB([]*transforms.Page{listing, postOne, postTwo})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}
	tmpl := template.Must(template.New("").Parse(`{{ define "page" }}page{{ end }}{{ define "list" }}list{{ end }}`))

	pages, err := ComputePageQueries(site, []*transforms.Page{listing, postOne, postTwo}, tmpl, db)
	if err != nil {
		t.Fatalf("ComputePageQueries failed: %v", err)
	}

	var page1, page2 *transforms.Page
	for _, page := range pages {
		switch page.URLPath {
		case "posts/01":
			page1 = page
		case "posts/02":
			page2 = page
		}
	}

	if page1 == nil || page2 == nil {
		t.Fatalf("expected posts/01 and posts/02 pages, got %#v", pages)
	}
	if len(page1.Aliases) != 1 || page1.Aliases[0] != "posts" {
		t.Fatalf("unexpected page1 aliases: %#v", page1.Aliases)
	}
	if page1.Pagination == nil || page1.Pagination.NextURL != "/posts/02" {
		t.Fatalf("unexpected page1 pagination: %#v", page1.Pagination)
	}
	if page2.Pagination == nil || page2.Pagination.PrevURL != "/posts/01" {
		t.Fatalf("unexpected page2 pagination: %#v", page2.Pagination)
	}
}

func TestBuildQueryDBPageLinksUsesCanonicalTypes(t *testing.T) {
	t.Parallel()

	source := &transforms.Page{
		Title:      "Source",
		SourcePath: "content/source.md",
		URLPath:    "source",
		Slug:       "source",
	}
	target := &transforms.Page{
		Title:      "Target",
		SourcePath: "content/target.md",
		URLPath:    "target",
		Slug:       "target",
	}
	source.Links = []transforms.PageLink{
		{
			RawTarget: "target",
			Label:     "Target",
			Target:    target,
		},
		{
			RawTarget: "missing",
			Label:     "Missing",
		},
	}

	db, err := BuildQueryDB([]*transforms.Page{source, target})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	result, err := queryResult(db, "select Source.URLPath as source_url, Target.URLPath as target_url from page_links order by RawTarget asc")
	if err != nil {
		t.Fatalf("page_links query failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("unexpected row count: %d", len(result.Rows))
	}
	if got := result.Rows[0]["source_url"]; got != "source" {
		t.Fatalf("unexpected source URL: %#v", got)
	}
	if got := result.Rows[0]["target_url"]; got != nil {
		t.Fatalf("expected nil target URL for unresolved link, got %#v", got)
	}
	if got := result.Rows[1]["target_url"]; got != "target" {
		t.Fatalf("unexpected target URL: %#v", got)
	}
	if got := source.Links[0].Source; got != source {
		t.Fatalf("expected canonical source page on first link, got %#v", got)
	}
}

func TestBuildQueryDBPageAssetsUsesCanonicalTypes(t *testing.T) {
	t.Parallel()

	page := &transforms.Page{
		Title:      "Asset Owner",
		SourcePath: "content/page.md",
		URLPath:    "page",
		Slug:       "page",
		Assets: map[string]*transforms.PageAsset{
			"hero": {
				Key:       "hero",
				Target:    "_assets/hero.png",
				URL:       "/_assets/hero.png",
				MediaType: "image/png",
			},
		},
	}

	db, err := BuildQueryDB([]*transforms.Page{page})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	result, err := queryResult(db, "select Owner.URLPath as owner_url, Key, MediaType from page_assets")
	if err != nil {
		t.Fatalf("page_assets query failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("unexpected row count: %d", len(result.Rows))
	}
	if got := result.Rows[0]["owner_url"]; got != "page" {
		t.Fatalf("unexpected owner URL: %#v", got)
	}
	if got := page.Assets["hero"].Owner; got != page {
		t.Fatalf("expected canonical owner page on asset, got %#v", got)
	}
}

func TestBuildQueryDBDoesNotRegisterSiteTable(t *testing.T) {
	t.Parallel()

	db, err := BuildQueryDB([]*transforms.Page{})
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	if _, err := queryResult(db, "select * from site"); err == nil {
		t.Fatalf("expected querying removed site table to fail")
	}
}
