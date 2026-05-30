package build

import (
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"

	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/structql"
)

type queryPageRow struct {
	transforms.PageTmpl
	Page transforms.PageTmpl `structql:"_page"`
}

type queryTagRow struct {
	Tag       string
	TagSlug   string
	PageSlug  string
	PagePath  string
	PageTitle string
	Section   string
	Draft     bool
	PubDate   time.Time
	Weight    int
	Page      transforms.PageTmpl `structql:"_page"`
}

func buildDB(pages []*transforms.Page, dataTables []dataTable) (*structql.DB, error) {
	db := structql.NewDB()

	pageRows := make([]queryPageRow, 0, len(pages))
	tagRows := make([]queryTagRow, 0)
	for _, page := range pages {
		if page.Error != nil {
			continue
		}
		tmplPage := page.Tmpl()
		pageRows = append(pageRows, newQueryPageRow(&tmplPage))
		for _, tag := range page.Tags {
			tagRows = append(tagRows, newQueryTagRow(tag, &tmplPage))
		}
	}

	table, err := structql.BuildTable(pageRows)
	if err != nil {
		return nil, fmt.Errorf(`build table "pages": %w`, err)
	}
	if err := db.Register("pages", table); err != nil {
		return nil, fmt.Errorf(`register table "pages": %w`, err)
	}

	tagTable, err := structql.BuildTable(tagRows)
	if err != nil {
		return nil, fmt.Errorf(`build table "tags": %w`, err)
	}
	if err := db.Register("tags", tagTable); err != nil {
		return nil, fmt.Errorf(`register table "tags": %w`, err)
	}

	if err := registerDataTables(db, dataTables); err != nil {
		return nil, err
	}

	return db, nil
}

func newQueryPageRow(page *transforms.PageTmpl) queryPageRow {
	if page == nil {
		return queryPageRow{}
	}

	return queryPageRow{
		PageTmpl: *page,
		Page:     *page,
	}
}

func newQueryTagRow(tag string, page *transforms.PageTmpl) queryTagRow {
	if page == nil {
		return queryTagRow{Tag: tag, TagSlug: tagSlug(tag)}
	}

	return queryTagRow{
		Tag:       tag,
		TagSlug:   tagSlug(tag),
		PageSlug:  page.Slug,
		PagePath:  page.Path,
		PageTitle: page.Title,
		Section:   page.Section,
		Draft:     page.Draft,
		PubDate:   page.PubDate,
		Weight:    page.Weight,
		Page:      *page,
	}
}

func tagSlug(tag string) string {
	tag = strings.TrimSpace(tag)
	var b strings.Builder
	lastSep := false
	for _, r := range strings.ToLower(tag) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastSep = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastSep = false
		case r == '-' || r == '_':
			if b.Len() == 0 || lastSep {
				continue
			}
			b.WriteRune(r)
			lastSep = r == '-'
		case unicode.IsSpace(r) || unicode.IsControl(r):
			if b.Len() == 0 || lastSep {
				continue
			}
			b.WriteByte('-')
			lastSep = true
		default:
			if b.Len() == 0 || lastSep {
				continue
			}
			b.WriteByte('-')
			lastSep = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func QueryFuncMap(db *structql.DB) template.FuncMap {
	return template.FuncMap{
		"query":      func(sql string, args ...any) ([]map[string]any, error) { return query(db, sql, args...) },
		"queryRow":   func(sql string, args ...any) (map[string]any, error) { return queryRow(db, sql, args...) },
		"queryPages": func(sql string, args ...any) ([]any, error) { return queryPages(db, sql, args...) },
		"queryPage":  func(sql string, args ...any) (any, error) { return queryPage(db, sql, args...) },
	}
}

func query(db *structql.DB, sql string, args ...any) ([]map[string]any, error) {
	result, err := db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", sql, err)
	}
	return result.Maps(), nil
}

func queryRow(db *structql.DB, sql string, args ...any) (map[string]any, error) {
	rows, err := query(db, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func queryPages(db *structql.DB, sql string, args ...any) ([]any, error) {
	result, err := db.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query %q: %w", sql, err)
	}

	pageCol := -1
	for i, col := range result.Columns {
		if strings.EqualFold(col.Name, "_page") {
			pageCol = i
			break
		}
	}
	if pageCol < 0 {
		return nil, fmt.Errorf("queryPages requires the result to include the _page column")
	}

	pages := make([]any, 0, len(result.Rows))
	for rowIdx, row := range result.Rows {
		if pageCol >= len(row) {
			return nil, fmt.Errorf("query result row %d is missing _page", rowIdx)
		}

		switch value := row[pageCol].(type) {
		case *transforms.PageTmpl:
			if value == nil {
				return nil, fmt.Errorf("query result expected _page to be a non-nil *PageTmpl in row %d", rowIdx)
			}
			pages = append(pages, value)
		case transforms.PageTmpl:
			pages = append(pages, value)
		default:
			return nil, fmt.Errorf("query result expected _page to be a PageTmpl or *PageTmpl in row %d", rowIdx)
		}
	}

	return pages, nil
}

func queryPage(db *structql.DB, sql string, args ...any) (any, error) {
	pages, err := queryPages(db, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}
	return pages[0], nil
}
