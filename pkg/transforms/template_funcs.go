package transforms

import (
	"fmt"
	"html/template"
	"maps"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
)

func DefaultTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"where":    TemplateFuncWhere,
		"whereEq":  TemplateFuncWhereEq,
		"whereNe":  TemplateFuncWhereNe,
		"whereHas": TemplateFuncWhereHas,
		"whereIn":  TemplateFuncWhereIn,
		"sort":     TemplateFuncSortBy,
		"limit":    TemplateFuncLimit,
		"offset":   TemplateFuncOffset,
		"first":    TemplateFuncFirst,
		"last":     TemplateFuncLast,
		"groupBy":  TemplateFuncGroupBy,
		"datefmt":  TemplateFuncDateFmt,
		"default":  TemplateFuncDefault,
		"uniq":     TemplateFuncUniq,
		"slugify":  TemplateFuncSlugify,
		"dict":     TemplateFuncDict,
		"merge":    TemplateFuncMerge,
	}
}

// TemplateFuncWhere filters pages using the legacy field mini-language.
func TemplateFuncWhere(field string, value any, pages []*PageLite) []*PageLite {
	switch field {
	case "Tags":
		return TemplateFuncWhereHas(field, value, pages)
	case "Tags:not":
		return TemplateFuncWhereNe("Tags", value, pages)
	case "Date:before":
		return filterPages(pages, func(page *PageLite) bool {
			v, ok := value.(time.Time)
			return ok && page.Date.Before(v)
		})
	case "Date:after":
		return filterPages(pages, func(page *PageLite) bool {
			v, ok := value.(time.Time)
			return ok && page.Date.After(v)
		})
	case "Updated:before":
		return filterPages(pages, func(page *PageLite) bool {
			v, ok := value.(time.Time)
			return ok && page.Updated.Before(v)
		})
	case "Updated:after":
		return filterPages(pages, func(page *PageLite) bool {
			v, ok := value.(time.Time)
			return ok && page.Updated.After(v)
		})
	default:
		return TemplateFuncWhereEq(field, value, pages)
	}
}

// TemplateFuncWhereEq filters pages whose field equals value.
func TemplateFuncWhereEq(field string, value any, pages []*PageLite) []*PageLite {
	return filterPages(pages, func(page *PageLite) bool {
		return pageFieldEquals(page, field, value)
	})
}

// TemplateFuncWhereNe filters pages whose field does not equal value.
func TemplateFuncWhereNe(field string, value any, pages []*PageLite) []*PageLite {
	return filterPages(pages, func(page *PageLite) bool {
		return !pageFieldEquals(page, field, value)
	})
}

// TemplateFuncWhereHas filters pages whose list-like field contains value.
func TemplateFuncWhereHas(field string, value any, pages []*PageLite) []*PageLite {
	return filterPages(pages, func(page *PageLite) bool {
		return pageFieldHas(page, field, value)
	})
}

// TemplateFuncWhereIn filters pages whose field matches any provided value.
func TemplateFuncWhereIn(field string, pages []*PageLite, values ...any) []*PageLite {
	return filterPages(pages, func(page *PageLite) bool {
		for _, value := range values {
			if pageFieldEquals(page, field, value) || pageFieldHas(page, field, value) {
				return true
			}
		}
		return false
	})
}

// TemplateFuncSortBy sorts pages by a field and order.
func TemplateFuncSortBy(field string, order string, pages []*PageLite) []*PageLite {
	out := slices.Clone(pages)

	if order != "asc" && order != "desc" {
		return []*PageLite{}
	}

	switch field {
	case "Title":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return strings.Compare(a.Title, b.Title)
		})
	case "Description":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return strings.Compare(a.Description, b.Description)
		})
	case "Section":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return strings.Compare(a.Section, b.Section)
		})
	case "Slug":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return strings.Compare(a.Slug, b.Slug)
		})
	case "Date":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return compareTimeAsc(a.Date, b.Date)
		})
	case "Updated":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return compareTimeAsc(a.Updated, b.Updated)
		})
	case "PubDate":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			return compareTimeAsc(a.PubDate, b.PubDate)
		})
	default:
		return []*PageLite{}
	}

	if order == "desc" {
		slices.Reverse(out)
	}

	return out
}

// TemplateFuncLimit limits the number of pages returned.
func TemplateFuncLimit(limit int, pages []*PageLite) []*PageLite {
	if limit <= 0 {
		return []*PageLite{}
	}
	if len(pages) <= limit {
		return pages
	}

	return pages[:limit]
}

// TemplateFuncOffset skips the first n pages.
func TemplateFuncOffset(offset int, pages []*PageLite) []*PageLite {
	if offset <= 0 {
		return pages
	}
	if offset >= len(pages) {
		return []*PageLite{}
	}
	return pages[offset:]
}

// TemplateFuncFirst returns the first page, or nil when empty.
func TemplateFuncFirst(pages []*PageLite) *PageLite {
	if len(pages) == 0 {
		return nil
	}
	return pages[0]
}

// TemplateFuncLast returns the last page, or nil when empty.
func TemplateFuncLast(pages []*PageLite) *PageLite {
	if len(pages) == 0 {
		return nil
	}
	return pages[len(pages)-1]
}

// TemplateFuncGroupBy groups pages by a supported field.
func TemplateFuncGroupBy(field string, pages []*PageLite) map[string][]*PageLite {
	out := make(map[string][]*PageLite)

	for _, page := range pages {
		switch field {
		case "Section":
			if page.Section != "" {
				out[page.Section] = append(out[page.Section], page)
			}
		case "Tags":
			for _, tag := range page.Tags {
				if tag == "" {
					continue
				}
				out[tag] = append(out[tag], page)
			}
		case "Year":
			if !page.Date.IsZero() {
				key := page.Date.Format("2006")
				out[key] = append(out[key], page)
			}
		case "YearMonth":
			if !page.Date.IsZero() {
				key := page.Date.Format("2006-01")
				out[key] = append(out[key], page)
			}
		case "Featured":
			key := fmt.Sprintf("%t", page.Featured)
			out[key] = append(out[key], page)
		case "Draft":
			key := fmt.Sprintf("%t", page.Draft)
			out[key] = append(out[key], page)
		}
	}

	return out
}

// TemplateFuncDateFmt formats a time using Go's time layout.
func TemplateFuncDateFmt(layout string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}

// TemplateFuncDefault returns fallback when value is empty.
func TemplateFuncDefault(fallback any, value any) any {
	if isTemplateEmpty(value) {
		return fallback
	}
	return value
}

// TemplateFuncUniq deduplicates strings while preserving order.
func TemplateFuncUniq(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}

// TemplateFuncSlugify creates a URL-safe slug.
func TemplateFuncSlugify(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	var b strings.Builder
	prevDash := false

	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case r == '/' || r == '-' || r == '_' || unicode.IsSpace(r):
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	slug := strings.Trim(b.String(), "-")
	slug = path.Clean(slug)
	slug = strings.Trim(slug, "/.")
	return slug
}

// TemplateFuncDict constructs a map from key/value pairs.
func TemplateFuncDict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict expects an even number of arguments")
	}

	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		out[key] = values[i+1]
	}

	return out, nil
}

// TemplateFuncMerge merges maps left-to-right.
func TemplateFuncMerge(mapsToMerge ...map[string]any) map[string]any {
	out := make(map[string]any)
	for _, m := range mapsToMerge {
		maps.Copy(out, m)
	}
	return out
}

func filterPages(pages []*PageLite, fn func(*PageLite) bool) []*PageLite {
	out := make([]*PageLite, 0, len(pages))
	for _, page := range pages {
		if fn(page) {
			out = append(out, page)
		}
	}
	return out
}

func compareTimeAsc(a, b time.Time) int {
	if a.After(b) {
		return +1
	} else if a.Before(b) {
		return -1
	}
	return 0
}

func pageFieldEquals(page *PageLite, field string, value any) bool {
	switch field {
	case "Title":
		v, ok := value.(string)
		return ok && page.Title == v
	case "Description":
		v, ok := value.(string)
		return ok && page.Description == v
	case "Section":
		v, ok := value.(string)
		return ok && page.Section == v
	case "Slug":
		v, ok := value.(string)
		return ok && page.Slug == v
	case "Featured":
		v, ok := value.(bool)
		return ok && page.Featured == v
	case "Draft":
		v, ok := value.(bool)
		return ok && page.Draft == v
	case "Date":
		v, ok := value.(time.Time)
		return ok && page.Date.Equal(v)
	case "Updated":
		v, ok := value.(time.Time)
		return ok && page.Updated.Equal(v)
	case "PubDate":
		v, ok := value.(time.Time)
		return ok && page.PubDate.Equal(v)
	case "Tags":
		return pageFieldHas(page, field, value)
	default:
		if key, ok := strings.CutPrefix(field, "Params."); ok {
			return page.Params[key] == value
		}
		return false
	}
}

func pageFieldHas(page *PageLite, field string, value any) bool {
	switch field {
	case "Tags":
		v, ok := value.(string)
		return ok && slices.Contains(page.Tags, v)
	default:
		if key, ok := strings.CutPrefix(field, "Params."); ok {
			switch values := page.Params[key].(type) {
			case []string:
				v, ok := value.(string)
				return ok && slices.Contains(values, v)
			case []any:
				for _, item := range values {
					if item == value {
						return true
					}
				}
			}
		}
		return false
	}
}

func isTemplateEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []*PageLite:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case time.Time:
		return v.IsZero()
	case bool:
		return !v
	case int:
		return v == 0
	case int8:
		return v == 0
	case int16:
		return v == 0
	case int32:
		return v == 0
	case int64:
		return v == 0
	case uint:
		return v == 0
	case uint8:
		return v == 0
	case uint16:
		return v == 0
	case uint32:
		return v == 0
	case uint64:
		return v == 0
	case float32:
		return v == 0
	case float64:
		return v == 0
	default:
		return false
	}
}
