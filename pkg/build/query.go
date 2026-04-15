package build

import (
	"fmt"
	"html/template"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/structql"
)

type pageQueryRow struct {
	Page        any `structql:"_page"`
	SourcePath  string
	ContentPath string
	URLPath     string
	OutputPath  string
	Template    string
	Slug        string
	Canon       string
	Aliases     []string
	Weight      int
	Title       string
	Description string
	Section     string
	Tags        []string
	Params      map[string]any
	Headers     map[string]string
	Date        time.Time
	Updated     time.Time
	PubDate     time.Time
	Featured    bool
	Draft       bool
}

type siteQueryRow struct {
	Title          string
	Description    string
	URL            string
	Params         map[string]any
	ConfigPath     string
	IsDev          bool
	BuildTime      time.Time
	GitAvailable   bool
	GitRepoRoot    string
	GitDir         string
	GitBranch      string
	GitCommitHash  string
	GitShortHash   string
	GitDirty       bool
	PageCount      int
	PublishedCount int
	DraftCount     int
}

type pageLinkQueryRow struct {
	SourcePage    any `structql:"_source_page"`
	TargetPage    any `structql:"_target_page"`
	SourcePath    string
	SourceURLPath string
	SourceSlug    string
	RawTarget     string
	Fragment      string
	Label         string
	Embed         bool
	Resolved      bool
	TargetPath    string
	TargetURLPath string
	TargetSlug    string
}

type pageAssetQueryRow struct {
	Page         any `structql:"_page"`
	OwnerPath    string
	OwnerURLPath string
	OwnerSlug    string
	Key          string
	Source       string
	Target       string
	URL          string
	Hash         string
	Size         int64
	MediaType    string
	Standalone   bool
}

func BuildQueryDB(site *transforms.Site, pages []*transforms.Page) (*structql.DB, error) {
	db := structql.NewDB()

	if err := registerPagesTable(db, pages); err != nil {
		return nil, err
	}
	if err := registerSiteTable(db, site, pages); err != nil {
		return nil, err
	}
	if err := registerPageLinksTable(db, pages); err != nil {
		return nil, err
	}
	if err := registerPageAssetsTable(db, pages); err != nil {
		return nil, err
	}

	return db, nil
}

func QueryFuncMap(db *structql.DB) template.FuncMap {
	return template.FuncMap{
		"query": func(sql string, args ...any) (*transforms.QueryResult, error) {
			return queryResult(db, sql, args...)
		},
	}
}

func registerPagesTable(db *structql.DB, pages []*transforms.Page) error {
	rows := make([]pageQueryRow, 0, len(pages))
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		rows = append(rows, pageQueryRow{
			Page:        page,
			SourcePath:  page.SourcePath,
			ContentPath: page.ContentPath,
			URLPath:     page.URLPath,
			OutputPath:  page.OutputPath,
			Template:    page.Template,
			Slug:        page.Slug,
			Canon:       page.Canon,
			Aliases:     slices.Clone(page.Aliases),
			Weight:      page.Weight,
			Title:       page.Title,
			Description: page.Description,
			Section:     page.Section,
			Tags:        slices.Clone(page.Tags),
			Params:      maps.Clone(page.Params),
			Headers:     maps.Clone(page.Headers),
			Date:        page.Date,
			Updated:     page.Updated,
			PubDate:     page.PubDate,
			Featured:    page.Featured,
			Draft:       page.Draft,
		})
	}

	return registerTable(db, "pages", rows)
}

func registerSiteTable(db *structql.DB, site *transforms.Site, pages []*transforms.Page) error {
	rows := []siteQueryRow{}
	if site != nil {
		rows = append(rows, siteQueryRow{
			Title:          site.Title,
			Description:    site.Description,
			URL:            site.URL,
			Params:         maps.Clone(site.Params),
			ConfigPath:     site.Meta.ConfigPath,
			IsDev:          site.Meta.IsDev,
			BuildTime:      site.Meta.BuildTime,
			GitAvailable:   site.Meta.Git.Available,
			GitRepoRoot:    site.Meta.Git.RepoRoot,
			GitDir:         site.Meta.Git.GitDir,
			GitBranch:      site.Meta.Git.Branch,
			GitCommitHash:  site.Meta.Git.CommitHash,
			GitShortHash:   site.Meta.Git.ShortHash,
			GitDirty:       site.Meta.Git.Dirty,
			PageCount:      len(pages),
			PublishedCount: countPublishedPages(pages),
			DraftCount:     countDraftPages(pages),
		})
	}

	return registerTable(db, "site", rows)
}

func registerPageLinksTable(db *structql.DB, pages []*transforms.Page) error {
	rows := make([]pageLinkQueryRow, 0)
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		for _, link := range page.Links {
			row := pageLinkQueryRow{
				SourcePage:    page,
				TargetPage:    link.Target,
				SourcePath:    page.SourcePath,
				SourceURLPath: page.URLPath,
				SourceSlug:    page.Slug,
				RawTarget:     link.RawTarget,
				Fragment:      link.Fragment,
				Label:         link.Label,
				Embed:         link.Embed,
				Resolved:      link.Resolved(),
			}
			if link.Target != nil {
				row.TargetPath = link.Target.SourcePath
				row.TargetURLPath = link.Target.URLPath
				row.TargetSlug = link.Target.Slug
			}
			rows = append(rows, row)
		}
	}

	return registerTable(db, "page_links", rows)
}

func registerPageAssetsTable(db *structql.DB, pages []*transforms.Page) error {
	rows := make([]pageAssetQueryRow, 0)
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		for _, asset := range page.Assets {
			if asset == nil {
				continue
			}
			rows = append(rows, pageAssetQueryRow{
				Page:         page,
				OwnerPath:    page.SourcePath,
				OwnerURLPath: page.URLPath,
				OwnerSlug:    page.Slug,
				Key:          asset.Key,
				Source:       asset.Source,
				Target:       asset.Target,
				URL:          asset.URL,
				Hash:         asset.Hash,
				Size:         asset.Size,
				MediaType:    asset.MediaType,
				Standalone:   asset.Standalone,
			})
		}
	}

	return registerTable(db, "page_assets", rows)
}

func registerTable[T any](db *structql.DB, name string, rows []T) error {
	table, err := structql.BuildTable(rows)
	if err != nil {
		return fmt.Errorf("build table %q: %w", name, err)
	}
	if err := db.Register(name, table); err != nil {
		return fmt.Errorf("register table %q: %w", name, err)
	}
	return nil
}

func countPublishedPages(pages []*transforms.Page) int {
	count := 0
	for _, page := range pages {
		if page == nil || page.HasError() || page.Draft {
			continue
		}
		count++
	}
	return count
}

func countDraftPages(pages []*transforms.Page) int {
	count := 0
	for _, page := range pages {
		if page == nil || page.HasError() || !page.Draft {
			continue
		}
		count++
	}
	return count
}

func runQuery(db *structql.DB, sql string, args ...any) (*structql.Result, error) {
	if db == nil {
		return nil, fmt.Errorf("query engine unavailable")
	}

	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	result, err := db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", sql, err)
	}
	return result, nil
}

func queryResult(db *structql.DB, sql string, args ...any) (*transforms.QueryResult, error) {
	result, err := runQuery(db, sql, args...)
	if err != nil {
		return nil, err
	}
	return buildQueryResult(result)
}

func buildQueryResult(result *structql.Result) (*transforms.QueryResult, error) {
	if result == nil {
		return &transforms.QueryResult{}, nil
	}

	rows := result.Maps()
	pages, err := resultPages(result)
	if err != nil {
		return nil, err
	}

	return &transforms.QueryResult{
		Rows:  rows,
		Pages: pages,
	}, nil
}

func resultPages(result *structql.Result) ([]*transforms.Page, error) {
	if result == nil {
		return nil, nil
	}

	pageColumn := -1
	for i, col := range result.Columns {
		if strings.EqualFold(strings.TrimSpace(col.Name), "_page") {
			pageColumn = i
			break
		}
	}
	if pageColumn < 0 {
		return nil, nil
	}

	out := make([]*transforms.Page, 0, len(result.Rows))
	for rowIdx, row := range result.Rows {
		if pageColumn >= len(row) {
			return nil, fmt.Errorf("query result row %d is missing _page", rowIdx)
		}

		page, ok := row[pageColumn].(*transforms.Page)
		if !ok || page == nil {
			return nil, fmt.Errorf("query result expected _page to be a *Page in row %d", rowIdx)
		}
		out = append(out, page)
	}

	return out, nil
}

func ComputeSiteQueries(db *structql.DB, defs map[string]config.ConfigSiteQuery) (map[string]*transforms.QueryResult, error) {
	if len(defs) == 0 {
		return nil, nil
	}

	out := make(map[string]*transforms.QueryResult, len(defs))
	for key, def := range defs {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("site query key cannot be empty")
		}
		result, err := queryResult(db, def.Query)
		if err != nil {
			return nil, fmt.Errorf("site query %q: %w", key, err)
		}
		out[key] = result
	}

	return out, nil
}

type PaginationPlan struct {
	Key      string
	Def      transforms.PageQueryDef
	Variants []*transforms.Page
}

func ComputePageQueries(site *transforms.Site, pages []*transforms.Page, tmpl *template.Template, db *structql.DB) ([]*transforms.Page, error) {
	out := make([]*transforms.Page, 0, len(pages))

	for _, page := range pages {
		if page == nil {
			continue
		}
		if page.HasError() {
			out = append(out, page)
			continue
		}

		pageQueries, plan, err := computeQueriesForPage(site, page, tmpl, db)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.SourcePath, err)
		}
		page.Queries = pageQueries
		out = append(out, page)

		if plan != nil {
			out = append(out, plan.Variants...)
		}
	}

	return out, nil
}

func computeQueriesForPage(site *transforms.Site, page *transforms.Page, tmpl *template.Template, db *structql.DB) (map[string]*transforms.QueryResult, *PaginationPlan, error) {
	frontmatterQueries := map[string]transforms.PageQueryDef(nil)
	dataQueries := map[string]transforms.PageQueryDef(nil)
	if page.Source.FrontmatterDoc != nil {
		frontmatterQueries = page.Source.FrontmatterDoc.Meta.Queries
	}
	if page.Source.DataDoc != nil {
		dataQueries = page.Source.DataDoc.Meta.Queries
	}
	if len(frontmatterQueries) == 0 && len(dataQueries) == 0 {
		return nil, nil, nil
	}

	defs := pageQueryDefs(page)
	if len(defs) == 0 {
		return nil, nil, nil
	}

	results := make(map[string]*transforms.QueryResult, len(defs))
	var paginationKey string
	var paginationDef transforms.PageQueryDef

	for key, def := range defs {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, nil, fmt.Errorf("page query key cannot be empty")
		}

		result, err := queryResult(db, def.Query, referencedPageQueryArgs(def.Query, page)...)
		if err != nil {
			return nil, nil, fmt.Errorf("page query %q: %w", key, err)
		}
		results[key] = result

		if !def.Paginate {
			continue
		}
		if paginationKey != "" {
			return nil, nil, fmt.Errorf("multiple paginated page queries are not allowed (%q, %q)", paginationKey, key)
		}
		paginationKey = key
		paginationDef = def
	}

	if paginationKey == "" {
		return results, nil, nil
	}

	plan, err := buildPaginationPlan(site, page, tmpl, paginationKey, paginationDef, results)
	if err != nil {
		return nil, nil, err
	}
	return results, plan, nil
}

func pageQueryDefs(page *transforms.Page) map[string]transforms.PageQueryDef {
	if page == nil {
		return nil
	}
	if page.Source.FrontmatterDoc != nil {
		return clonePageQueryDefMap(page.Source.FrontmatterDoc.Meta.Queries)
	}
	if page.Source.DataDoc != nil {
		return clonePageQueryDefMap(page.Source.DataDoc.Meta.Queries)
	}
	return nil
}

func buildPaginationPlan(site *transforms.Site, page *transforms.Page, tmpl *template.Template, key string, def transforms.PageQueryDef, results map[string]*transforms.QueryResult) (*PaginationPlan, error) {
	if strings.TrimSpace(def.Template) == "" {
		return nil, fmt.Errorf("paginated page query %q requires template", key)
	}
	if def.PageSize <= 0 {
		return nil, fmt.Errorf("paginated page query %q requires page_size > 0", key)
	}
	if tmpl == nil || tmpl.Lookup(def.Template) == nil {
		return nil, fmt.Errorf("paginated page query %q template %q not found", key, def.Template)
	}

	baseResult := results[key]
	if baseResult == nil {
		baseResult = &transforms.QueryResult{}
	}
	if !queryResultHasPages(baseResult) {
		return nil, fmt.Errorf("paginated page query %q requires the result to include the _page column", key)
	}

	totalItems := len(baseResult.Rows)
	totalPages := totalItems / def.PageSize
	if totalItems%def.PageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	page.Template = def.Template
	page.Pagination = buildPaginationState(page.URLPath, key, 1, def.PageSize, totalItems, totalPages)
	page.Queries = cloneQueryResults(results)
	page.Queries[key] = sliceQueryResult(baseResult, 0, def.PageSize)

	variants := make([]*transforms.Page, 0, max(0, totalPages-1))
	for pageNum := 2; pageNum <= totalPages; pageNum++ {
		start := (pageNum - 1) * def.PageSize
		end := min(start+def.PageSize, totalItems)

		variant := clonePage(page)
		variant.URLPath = paginatedURLPath(page.URLPath, pageNum)
		variant.OutputPath = pathutil.OutputPathForURLPath(variant.URLPath)
		variant.Canon = mustCanonicalPageURL(site, variant.URLPath)
		variant.Template = def.Template
		variant.Aliases = nil
		variant.Pagination = buildPaginationState(page.URLPath, key, pageNum, def.PageSize, totalItems, totalPages)
		variant.Queries = cloneQueryResults(results)
		variant.Queries[key] = sliceQueryResult(baseResult, start, end)
		variants = append(variants, variant)
	}

	return &PaginationPlan{Key: key, Def: def, Variants: variants}, nil
}

func pageQueryArgs(page *transforms.Page) map[string]any {
	if page == nil {
		return nil
	}
	return map[string]any{
		"page_source_path":  page.SourcePath,
		"page_content_path": page.ContentPath,
		"page_url_path":     page.URLPath,
		"page_output_path":  page.OutputPath,
		"page_template":     page.Template,
		"page_slug":         page.Slug,
		"page_canon":        page.Canon,
		"page_title":        page.Title,
		"page_description":  page.Description,
		"page_section":      page.Section,
		"page_weight":       page.Weight,
		"page_featured":     page.Featured,
		"page_draft":        page.Draft,
		"page_date":         page.Date,
		"page_updated":      page.Updated,
		"page_pub_date":     page.PubDate,
	}
}

func cloneQueryResults(in map[string]*transforms.QueryResult) map[string]*transforms.QueryResult {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]*transforms.QueryResult, len(in))
	for key, value := range in {
		out[key] = cloneQueryResult(value)
	}
	return out
}

func cloneQueryResult(in *transforms.QueryResult) *transforms.QueryResult {
	if in == nil {
		return nil
	}
	out := &transforms.QueryResult{
		Rows:  make([]map[string]any, 0, len(in.Rows)),
		Pages: slices.Clone(in.Pages),
	}
	for _, row := range in.Rows {
		out.Rows = append(out.Rows, maps.Clone(row))
	}
	return out
}

func sliceQueryResult(in *transforms.QueryResult, start, end int) *transforms.QueryResult {
	if in == nil {
		return &transforms.QueryResult{}
	}
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(in.Rows) {
		start = len(in.Rows)
	}
	if end > len(in.Rows) {
		end = len(in.Rows)
	}

	out := &transforms.QueryResult{
		Rows: make([]map[string]any, 0, end-start),
	}
	if queryResultHasPages(in) {
		out.Pages = slices.Clone(in.Pages[start:end])
	}
	for _, row := range in.Rows[start:end] {
		out.Rows = append(out.Rows, maps.Clone(row))
	}
	return out
}

func clonePage(page *transforms.Page) *transforms.Page {
	if page == nil {
		return nil
	}
	out := *page
	out.Aliases = slices.Clone(page.Aliases)
	out.Tags = slices.Clone(page.Tags)
	out.Queries = cloneQueryResults(page.Queries)
	out.Pagination = clonePagination(page.Pagination)
	return &out
}

func clonePagination(in *transforms.PaginationState) *transforms.PaginationState {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func buildPaginationState(baseURLPath, key string, pageNumber, pageSize, totalItems, totalPages int) *transforms.PaginationState {
	state := &transforms.PaginationState{
		QueryKey:   key,
		PageNumber: pageNumber,
		PageSize:   pageSize,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
	if pageNumber > 1 {
		state.HasPrev = true
		state.PrevPageNumber = pageNumber - 1
		state.PrevURL = pathutil.EnsureLeadingSlash(paginatedURLPath(baseURLPath, pageNumber-1))
	}
	if pageNumber < totalPages {
		state.HasNext = true
		state.NextPageNumber = pageNumber + 1
		state.NextURL = pathutil.EnsureLeadingSlash(paginatedURLPath(baseURLPath, pageNumber+1))
	}
	return state
}

func paginatedURLPath(baseURLPath string, pageNumber int) string {
	baseURLPath = strings.Trim(baseURLPath, "/")
	if pageNumber <= 1 {
		return baseURLPath
	}
	return path.Join(baseURLPath, "page", strconv.Itoa(pageNumber))
}

func mustCanonicalPageURL(site *transforms.Site, urlPath string) string {
	if site == nil {
		return ""
	}
	canon, err := pathutil.CanonicalPageURL(site.URL, urlPath)
	if err != nil {
		return ""
	}
	return canon
}

func referencedPageQueryArgs(sql string, page *transforms.Page) []any {
	_, named, err := structql.RequiredArgs(sql)
	if err != nil {
		return nil
	}

	available := pageQueryArgs(page)
	if len(available) == 0 || len(named) == 0 {
		return nil
	}

	args := make([]any, 0, len(named))
	for _, name := range named {
		if value, ok := available[name]; ok {
			args = append(args, structql.Named(name, value))
		}
	}
	return args
}

func queryResultHasPages(result *transforms.QueryResult) bool {
	if result == nil {
		return false
	}
	return result.Pages != nil
}

func clonePageQueryDefMap(in map[string]transforms.PageQueryDef) map[string]transforms.PageQueryDef {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]transforms.PageQueryDef, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
