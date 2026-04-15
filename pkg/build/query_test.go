package build

import (
	"html/template"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/transforms"
)

func TestQueryResultAndFirst(t *testing.T) {
	site, pages := testQueryFixture()

	db, err := BuildQueryDB(site, pages)
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	result, err := queryResult(
		db,
		"select Title, URLPath from pages where Draft = ? and contains(Tags, ?) order by Weight desc limit ?",
		false,
		"go",
		2,
	)
	if err != nil {
		t.Fatalf("queryResult failed: %v", err)
	}

	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if got := result.Rows[0]["Title"]; got != "Latest Post" {
		t.Fatalf("expected first row title %q, got %#v", "Latest Post", got)
	}
	if got := result.Rows[1]["URLPath"]; got != "posts/first" {
		t.Fatalf("expected second row URLPath %q, got %#v", "posts/first", got)
	}

	row := transforms.TemplateFuncFirst(result)
	rowMap, ok := row.(map[string]any)
	if !ok {
		t.Fatalf("expected first row to be a map, got %T", row)
	}

	siteResult, err := queryResult(db, "select Title, DraftCount from site")
	if err != nil {
		t.Fatalf("queryResult failed: %v", err)
	}
	siteRow, ok := transforms.TemplateFuncFirst(siteResult).(map[string]any)
	if !ok || siteRow == nil {
		t.Fatal("expected one site row")
	}
	if got := siteRow["Title"]; got != "Test Site" {
		t.Fatalf("expected site title %q, got %#v", "Test Site", got)
	}
	if got := siteRow["DraftCount"]; got != 1 {
		t.Fatalf("expected draft count %d, got %#v", 1, got)
	}
	if got := rowMap["Title"]; got != "Latest Post" {
		t.Fatalf("expected first helper row title %q, got %#v", "Latest Post", got)
	}
}

func TestTemplateQueryResultAsPages(t *testing.T) {
	site, pages := testQueryFixture()

	db, err := BuildQueryDB(site, pages)
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	result, err := queryResult(
		db,
		"select * from pages where Section = ? and Draft = ? order by Weight desc",
		"posts",
		false,
	)
	if err != nil {
		t.Fatalf("queryResult failed: %v", err)
	}

	matched, err := transforms.TemplateFuncAsPages(result)
	if err != nil {
		t.Fatalf("TemplateFuncAsPages failed: %v", err)
	}

	if len(matched) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(matched))
	}
	if matched[0] != pages[1] {
		t.Fatalf("expected first page pointer to be pages[1]")
	}
	if matched[1] != pages[0] {
		t.Fatalf("expected second page pointer to be pages[0]")
	}

	one, err := transforms.TemplateFuncAsPage(result)
	if err != nil {
		t.Fatalf("TemplateFuncAsPage failed: %v", err)
	}
	if one != pages[1] {
		t.Fatalf("expected first page pointer to be pages[1], got %#v", one)
	}

	projected, err := queryResult(db, "select Title from pages")
	if err != nil {
		t.Fatalf("queryResult failed: %v", err)
	}
	if _, err := transforms.TemplateFuncAsPages(projected); err == nil {
		t.Fatal("expected asPages to reject projections without _page")
	}
}

func TestQueryJoinsDerivedTables(t *testing.T) {
	site, pages := testQueryFixture()

	db, err := BuildQueryDB(site, pages)
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	result, err := queryResult(
		db,
		"select p.Title, a.Key, l.TargetSlug from pages p left join page_assets a on p.URLPath = a.OwnerURLPath left join page_links l on p.URLPath = l.SourceURLPath where p.Slug = ?",
		"first-post",
	)
	if err != nil {
		t.Fatalf("queryResult failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 joined row, got %d", len(result.Rows))
	}
	if got := result.Rows[0]["Key"]; got != "hero.png" {
		t.Fatalf("expected asset key %q, got %#v", "hero.png", got)
	}
	if got := result.Rows[0]["TargetSlug"]; got != "latest-post" {
		t.Fatalf("expected link target slug %q, got %#v", "latest-post", got)
	}
}

func TestComputeSiteQueries(t *testing.T) {
	site, pages := testQueryFixture()

	db, err := BuildQueryDB(site, pages)
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	results, err := ComputeSiteQueries(db, map[string]config.ConfigSiteQuery{
		"posts": {Query: "select * from pages where Section = 'posts' order by Weight desc"},
	})
	if err != nil {
		t.Fatalf("ComputeSiteQueries failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 site query result, got %d", len(results))
	}
	if got := len(results["posts"].Pages); got != 2 {
		t.Fatalf("expected 2 page-backed site query rows, got %d", got)
	}
	if got := results["posts"].Pages[0].Title; got != "Latest Post" {
		t.Fatalf("expected first site query page %q, got %q", "Latest Post", got)
	}
}

func TestComputePageQueriesWithPagination(t *testing.T) {
	site, pages := testQueryFixture()
	pages[0].Source = transforms.PageSource{
		FrontmatterDoc: &transforms.FrontmatterDoc{
			Meta: transforms.Frontmatter{
				Queries: map[string]transforms.PageQueryDef{
					"posts": {
						Query:    "select * from pages where Section = 'posts' and Draft = false order by Weight desc",
						Paginate: true,
						PageSize: 1,
						Template: "paginated",
					},
				},
			},
		},
	}

	db, err := BuildQueryDB(site, pages)
	if err != nil {
		t.Fatalf("BuildQueryDB failed: %v", err)
	}

	tmpl := template.Must(template.New("paginated").Parse(`{{define "paginated"}}ok{{end}}`))
	expanded, err := ComputePageQueries(site, pages, tmpl, db)
	if err != nil {
		t.Fatalf("ComputePageQueries failed: %v", err)
	}

	if len(expanded) != 4 {
		t.Fatalf("expected 4 pages after pagination expansion, got %d", len(expanded))
	}

	first := expanded[0]
	if first.Template != "paginated" {
		t.Fatalf("expected first page template %q, got %q", "paginated", first.Template)
	}
	if first.Pagination == nil || !first.Pagination.HasNext || first.Pagination.PageNumber != 1 {
		t.Fatalf("expected page 1 pagination state, got %#v", first.Pagination)
	}
	if got := first.Queries["posts"].Pages[0].Title; got != "Latest Post" {
		t.Fatalf("expected page 1 query slice to start with %q, got %q", "Latest Post", got)
	}

	variant := expanded[1]
	if variant.URLPath != "posts/first/page/2" {
		t.Fatalf("expected variant URLPath %q, got %q", "posts/first/page/2", variant.URLPath)
	}
	if variant.Pagination == nil || !variant.Pagination.HasPrev || variant.Pagination.PageNumber != 2 {
		t.Fatalf("expected page 2 pagination state, got %#v", variant.Pagination)
	}
	if got := variant.Queries["posts"].Pages[0].Title; got != "First Post" {
		t.Fatalf("expected page 2 query slice to contain %q, got %q", "First Post", got)
	}
}

func testQueryFixture() (*transforms.Site, []*transforms.Page) {
	first := &transforms.Page{
		SourcePath:  "content/posts/first.md",
		ContentPath: "posts/first.md",
		URLPath:     "posts/first",
		OutputPath:  "posts/first/index.html",
		Template:    "post",
		Slug:        "first-post",
		Canon:       "https://example.com/posts/first/",
		Title:       "First Post",
		Description: "First",
		Section:     "posts",
		Tags:        []string{"go", "intro"},
		Date:        time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		Updated:     time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		PubDate:     time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Params:      map[string]any{"series": "notes"},
		Headers:     map[string]string{"x-test": "one"},
		Weight:      10,
		Assets: map[string]*transforms.PageAsset{
			"hero.png": {
				Key:        "hero.png",
				Source:     "content/posts/hero.png",
				Target:     "_assets/hero.png",
				URL:        "/_assets/hero.png",
				Hash:       "abc123",
				Size:       42,
				MediaType:  "image/png",
				Standalone: true,
			},
		},
	}

	latest := &transforms.Page{
		SourcePath:  "content/posts/latest.md",
		ContentPath: "posts/latest.md",
		URLPath:     "posts/latest",
		OutputPath:  "posts/latest/index.html",
		Template:    "post",
		Slug:        "latest-post",
		Canon:       "https://example.com/posts/latest/",
		Title:       "Latest Post",
		Description: "Latest",
		Section:     "posts",
		Tags:        []string{"go", "advanced"},
		Date:        time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
		Updated:     time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		PubDate:     time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
		Params:      map[string]any{"series": "notes"},
		Headers:     map[string]string{"x-test": "two"},
		Weight:      20,
		Assets:      map[string]*transforms.PageAsset{},
	}

	draft := &transforms.Page{
		SourcePath:  "content/drafts/draft.md",
		ContentPath: "drafts/draft.md",
		URLPath:     "drafts/draft",
		OutputPath:  "drafts/draft/index.html",
		Template:    "page",
		Slug:        "draft-page",
		Canon:       "https://example.com/drafts/draft/",
		Title:       "Draft Page",
		Section:     "drafts",
		Tags:        []string{"scratch"},
		Date:        time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC),
		Draft:       true,
		Params:      map[string]any{"series": "internal"},
		Headers:     map[string]string{},
		Assets:      map[string]*transforms.PageAsset{},
	}

	first.Links = []transforms.PageLink{{
		RawTarget: "posts/latest",
		Label:     "Latest",
		Target:    latest,
	}}
	latest.Links = []transforms.PageLink{}
	draft.Links = []transforms.PageLink{}

	pages := []*transforms.Page{first, latest, draft}
	site := &transforms.Site{
		Title:       "Test Site",
		Description: "Fixture",
		URL:         "https://example.com",
		Params:      map[string]any{"author": "Oliver"},
		Meta: transforms.SiteMeta{
			ConfigPath: "shizuka.toml",
			IsDev:      true,
			BuildTime:  time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		},
	}

	return site, pages
}
