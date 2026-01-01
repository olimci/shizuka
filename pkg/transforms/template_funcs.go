package transforms

import (
	"slices"
	"strings"
	"time"
)

// TemplateFuncWhere filters pages based on a field and value.
func TemplateFuncWhere(field string, value any, pages []*PageLite) []*PageLite {
	out := make([]*PageLite, 0, len(pages))

	for _, page := range pages {
		var match bool
		switch field {
		case "Section":
			if v, ok := value.(string); ok {
				match = page.Section == v
			}
		case "Featured":
			if v, ok := value.(bool); ok {
				match = page.Featured == v
			}
		case "Draft":
			if v, ok := value.(bool); ok {
				match = page.Draft == v
			}
		case "Date:before":
			if v, ok := value.(time.Time); ok {
				match = page.Date.Before(v)
			}
		case "Date:after":
			if v, ok := value.(time.Time); ok {
				match = page.Date.After(v)
			}
		case "Updated:before":
			if v, ok := value.(time.Time); ok {
				match = page.Updated.Before(v)
			}
		case "Updated:after":
			if v, ok := value.(time.Time); ok {
				match = page.Updated.After(v)
			}
		case "Tags":
			if v, ok := value.(string); ok {
				match = slices.Contains(page.Tags, v)
			}
		case "Tags:not":
			if v, ok := value.(string); ok {
				match = !slices.Contains(page.Tags, v)
			}
		}

		if match {
			out = append(out, page)
		}
	}

	return out
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
			if a.Date.After(b.Date) {
				return -1
			} else if a.Date.Before(b.Date) {
				return +1
			}
			return 0
		})
	case "Updated":
		slices.SortStableFunc(out, func(a, b *PageLite) int {
			if a.Updated.After(b.Updated) {
				return -1
			} else if a.Updated.Before(b.Updated) {
				return +1
			}
			return 0
		})
	}

	if order == "desc" {
		slices.Reverse(out)
	}

	return out
}

// TemplateFuncLimit limits the number of pages returned.
func TemplateFuncLimit(limit int, pages []*PageLite) []*PageLite {
	if len(pages) <= limit {
		return pages
	}

	return pages[:limit]
}
