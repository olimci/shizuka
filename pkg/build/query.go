package build

import (
	"fmt"
	"html/template"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/structql"
)

func BuildQueryDB(pages []*transforms.Page) (*structql.DB, error) {
	db := structql.NewDB()

	if err := registerPagesTable(db, pages); err != nil {
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
	rows := make([]*transforms.Page, 0, len(pages))
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		page.QueryPage = page
		rows = append(rows, page)
	}

	return registerTable(db, "pages", rows)
}

func registerPageLinksTable(db *structql.DB, pages []*transforms.Page) error {
	rows := make([]transforms.PageLink, 0)
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		for i := range page.Links {
			if page.Links[i].Source == nil {
				page.Links[i].Source = page
			}
			rows = append(rows, page.Links[i])
		}
	}

	return registerTable(db, "page_links", rows)
}

func registerPageAssetsTable(db *structql.DB, pages []*transforms.Page) error {
	rows := make([]*transforms.PageAsset, 0)
	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}
		for _, asset := range page.Assets {
			if asset == nil {
				continue
			}
			if asset.Owner == nil {
				asset.Owner = page
			}
			rows = append(rows, asset)
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

		switch value := row[pageColumn].(type) {
		case *transforms.Page:
			if value == nil {
				return nil, fmt.Errorf("query result expected _page to be a non-nil *Page in row %d", rowIdx)
			}
			out = append(out, value)
		case transforms.Page:
			if value.QueryPage == nil {
				return nil, fmt.Errorf("query result expected _page value to carry QueryPage in row %d", rowIdx)
			}
			out = append(out, value.QueryPage)
		default:
			return nil, fmt.Errorf("query result expected _page to be a Page or *Page in row %d", rowIdx)
		}
	}

	return out, nil
}

type QueryExpansionPlan struct {
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

func computeQueriesForPage(site *transforms.Site, page *transforms.Page, tmpl *template.Template, db *structql.DB) (map[string]*transforms.QueryResult, *QueryExpansionPlan, error) {
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
	var expansionKey string
	var expansionDef transforms.PageQueryDef

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

		if !queryNeedsExpansion(def) {
			continue
		}
		if expansionKey != "" {
			return nil, nil, fmt.Errorf("multiple expanding page queries are not allowed (%q, %q)", expansionKey, key)
		}
		expansionKey = key
		expansionDef = def
	}

	if expansionKey == "" {
		return results, nil, nil
	}

	plan, err := buildQueryExpansionPlan(site, page, tmpl, expansionKey, expansionDef, results)
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
		return maps.Clone(page.Source.FrontmatterDoc.Meta.Queries)
	}
	if page.Source.DataDoc != nil {
		return maps.Clone(page.Source.DataDoc.Meta.Queries)
	}
	return nil
}

func buildQueryExpansionPlan(site *transforms.Site, page *transforms.Page, tmpl *template.Template, key string, def transforms.PageQueryDef, results map[string]*transforms.QueryResult) (*QueryExpansionPlan, error) {
	if strings.TrimSpace(def.Template) == "" {
		return nil, fmt.Errorf("expanding page query %q requires template", key)
	}
	if def.Paginate && def.PageSize <= 0 {
		return nil, fmt.Errorf("paginated page query %q requires page_size > 0", key)
	}
	if tmpl == nil || tmpl.Lookup(def.Template) == nil {
		return nil, fmt.Errorf("expanding page query %q template %q not found", key, def.Template)
	}

	baseResult := results[key]
	if baseResult == nil {
		baseResult = &transforms.QueryResult{}
	}
	if !queryResultHasPages(baseResult) {
		return nil, fmt.Errorf("expanding page query %q requires the result to include the _page column", key)
	}

	if strings.TrimSpace(def.GroupBy) == "" {
		return buildUngroupedExpansionPlan(site, page, key, def, results, baseResult), nil
	}
	return buildGroupedExpansionPlan(site, page, key, def, results, baseResult)
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
		"page_created":      page.Created,
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
	out.QueryPage = &out
	out.Aliases = slices.Clone(page.Aliases)
	out.Tags = slices.Clone(page.Tags)
	out.Queries = cloneQueryResults(page.Queries)
	out.Pagination = clonePagination(page.Pagination)
	out.Group = cloneGroup(page.Group)
	return &out
}

func clonePagination(in *transforms.PaginationState) *transforms.PaginationState {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneGroup(in *transforms.QueryGroupState) *transforms.QueryGroupState {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func buildPaginationState(baseURLPath, key, pageFormat string, redirectPageOne bool, pageNumber, pageSize, totalItems, totalPages int) *transforms.PaginationState {
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
		state.PrevURL = pathutil.EnsureLeadingSlash(paginatedURLPath(baseURLPath, pageFormat, redirectPageOne, pageNumber-1))
	}
	if pageNumber < totalPages {
		state.HasNext = true
		state.NextPageNumber = pageNumber + 1
		state.NextURL = pathutil.EnsureLeadingSlash(paginatedURLPath(baseURLPath, pageFormat, redirectPageOne, pageNumber+1))
	}
	return state
}

func paginatedURLPath(baseURLPath, pageFormat string, redirectPageOne bool, pageNumber int) string {
	baseURLPath = strings.Trim(baseURLPath, "/")
	if pageNumber <= 1 && !redirectPageOne {
		return baseURLPath
	}
	formatted, err := formatPathSegment(defaultPageFormat(pageFormat), pageNumber)
	if err != nil {
		return path.Join(baseURLPath, "page", strconv.Itoa(pageNumber))
	}
	return joinURLPath(baseURLPath, formatted)
}

func queryNeedsExpansion(def transforms.PageQueryDef) bool {
	return def.Paginate || strings.TrimSpace(def.GroupBy) != ""
}

func buildUngroupedExpansionPlan(site *transforms.Site, page *transforms.Page, key string, def transforms.PageQueryDef, results map[string]*transforms.QueryResult, baseResult *transforms.QueryResult) *QueryExpansionPlan {
	totalItems := len(baseResult.Rows)
	totalPages := pageCount(totalItems, def)
	pageFormat := defaultPageFormat(def.PageFormat)
	baseURLPath := strings.Trim(page.URLPath, "/")

	if def.RedirectPageOne {
		alias := baseURLPath
		page.URLPath = paginatedURLPath(baseURLPath, pageFormat, true, 1)
		page.OutputPath = pathutil.OutputPathForURLPath(page.URLPath)
		page.Canon = mustCanonicalPageURL(site, page.URLPath)
		page.Aliases = appendPageAlias(page.Aliases, alias, page.URLPath)
	}

	page.Template = def.Template
	page.Pagination = buildPaginationState(baseURLPath, key, pageFormat, def.RedirectPageOne, 1, effectivePageSize(def, totalItems), totalItems, totalPages)
	page.Group = nil
	page.Queries = cloneQueryResults(results)
	page.Queries[key] = sliceQueryResult(baseResult, 0, effectivePageSize(def, totalItems))

	variants := make([]*transforms.Page, 0, max(0, totalPages-1))
	for pageNum := 2; pageNum <= totalPages; pageNum++ {
		start := (pageNum - 1) * effectivePageSize(def, totalItems)
		end := min(start+effectivePageSize(def, totalItems), totalItems)

		variant := clonePage(page)
		variant.URLPath = paginatedURLPath(baseURLPath, pageFormat, def.RedirectPageOne, pageNum)
		variant.OutputPath = pathutil.OutputPathForURLPath(variant.URLPath)
		variant.Canon = mustCanonicalPageURL(site, variant.URLPath)
		variant.Template = def.Template
		variant.Aliases = nil
		variant.Pagination = buildPaginationState(baseURLPath, key, pageFormat, def.RedirectPageOne, pageNum, effectivePageSize(def, totalItems), totalItems, totalPages)
		variant.Queries = cloneQueryResults(results)
		variant.Queries[key] = sliceQueryResult(baseResult, start, end)
		variants = append(variants, variant)
	}

	return &QueryExpansionPlan{Key: key, Def: def, Variants: variants}
}

func buildGroupedExpansionPlan(site *transforms.Site, page *transforms.Page, key string, def transforms.PageQueryDef, results map[string]*transforms.QueryResult, baseResult *transforms.QueryResult) (*QueryExpansionPlan, error) {
	grouped, err := groupQueryResult(baseResult, def.GroupBy)
	if err != nil {
		return nil, fmt.Errorf("page query %q: %w", key, err)
	}

	pageFormat := defaultPageFormat(def.PageFormat)
	groupFormat := defaultGroupFormat(def.GroupFormat)
	baseURLPath := strings.Trim(page.URLPath, "/")
	variants := make([]*transforms.Page, 0)

	for _, group := range grouped {
		groupSegment, err := formatPathSegment(groupFormat, group.Value)
		if err != nil {
			return nil, fmt.Errorf("page query %q group %q: %w", key, def.GroupBy, err)
		}
		groupBaseURLPath := joinURLPath(baseURLPath, groupSegment)
		totalItems := len(group.Result.Rows)
		totalPages := pageCount(totalItems, def)
		pageSize := effectivePageSize(def, totalItems)

		for pageNum := 1; pageNum <= totalPages; pageNum++ {
			start := (pageNum - 1) * pageSize
			end := min(start+pageSize, totalItems)

			variant := clonePage(page)
			variant.Template = def.Template
			variant.Aliases = nil
			variant.Group = &transforms.QueryGroupState{
				QueryKey: key,
				Column:   def.GroupBy,
				Value:    group.Value,
				URLPath:  groupBaseURLPath,
			}
			variant.Pagination = buildPaginationState(groupBaseURLPath, key, pageFormat, def.RedirectPageOne, pageNum, pageSize, totalItems, totalPages)
			variant.Queries = cloneQueryResults(results)
			variant.Queries[key] = sliceQueryResult(group.Result, start, end)
			variant.URLPath = paginatedURLPath(groupBaseURLPath, pageFormat, def.RedirectPageOne, pageNum)
			if pageNum == 1 && def.RedirectPageOne {
				variant.Aliases = appendPageAlias(variant.Aliases, groupBaseURLPath, variant.URLPath)
			}
			variant.OutputPath = pathutil.OutputPathForURLPath(variant.URLPath)
			variant.Canon = mustCanonicalPageURL(site, variant.URLPath)
			variants = append(variants, variant)
		}
	}

	return &QueryExpansionPlan{Key: key, Def: def, Variants: variants}, nil
}

type groupedQueryResult struct {
	Value  any
	Result *transforms.QueryResult
}

func groupQueryResult(result *transforms.QueryResult, column string) ([]groupedQueryResult, error) {
	if result == nil {
		return nil, nil
	}

	type groupState struct {
		value any
		rows  []map[string]any
		pages []*transforms.Page
	}

	order := make([]string, 0)
	groups := make(map[string]*groupState)
	for i, row := range result.Rows {
		value, ok := lookupRowColumn(row, column)
		if !ok {
			return nil, fmt.Errorf("group_by column %q not found in query result", column)
		}
		key := fmt.Sprintf("%T:%v", value, value)
		state := groups[key]
		if state == nil {
			state = &groupState{value: value}
			groups[key] = state
			order = append(order, key)
		}
		state.rows = append(state.rows, maps.Clone(row))
		if i < len(result.Pages) {
			state.pages = append(state.pages, result.Pages[i])
		}
	}

	out := make([]groupedQueryResult, 0, len(order))
	for _, key := range order {
		state := groups[key]
		out = append(out, groupedQueryResult{
			Value: state.value,
			Result: &transforms.QueryResult{
				Rows:  state.rows,
				Pages: state.pages,
			},
		})
	}
	return out, nil
}

func lookupRowColumn(row map[string]any, column string) (any, bool) {
	for key, value := range row {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(column)) {
			return value, true
		}
	}
	return nil, false
}

func defaultPageFormat(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return "page/%d"
	}
	return format
}

func defaultGroupFormat(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return "%v"
	}
	return format
}

func pageCount(totalItems int, def transforms.PageQueryDef) int {
	pageSize := effectivePageSize(def, totalItems)
	totalPages := totalItems / pageSize
	if totalItems%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return totalPages
}

func effectivePageSize(def transforms.PageQueryDef, totalItems int) int {
	if def.Paginate && def.PageSize > 0 {
		return def.PageSize
	}
	if totalItems > 0 {
		return totalItems
	}
	return 1
}

func formatPathSegment(format string, value any) (string, error) {
	formatted := fmt.Sprintf(format, value)
	if strings.Contains(formatted, "%!") {
		return "", fmt.Errorf("invalid path format %q for value %T", format, value)
	}
	cleaned, err := pathutil.CleanURLPath(formatted)
	if err != nil {
		return "", err
	}
	if cleaned == "" {
		return "", fmt.Errorf("formatted path is empty")
	}
	return cleaned, nil
}

func joinURLPath(base, segment string) string {
	base = strings.Trim(base, "/")
	segment = strings.Trim(segment, "/")
	switch {
	case base == "":
		return segment
	case segment == "":
		return base
	default:
		return path.Join(base, segment)
	}
}

func appendPageAlias(aliases []string, alias, urlPath string) []string {
	alias = strings.Trim(alias, "/")
	urlPath = strings.Trim(urlPath, "/")
	if alias == "" || alias == urlPath {
		return aliases
	}
	if slices.Contains(aliases, alias) {
		return aliases
	}
	return append(aliases, alias)
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
