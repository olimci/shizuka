package transforms

import (
	"fmt"
	"html/template"
	"maps"
	"path"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/olimci/shizuka/pkg/utils/errutil"
)

func DefaultTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"discard":   TemplateFuncDiscard,
		"datefmt":   TemplateFuncDateFmt,
		"default":   TemplateFuncDefault,
		"uniq":      TemplateFuncUniq,
		"slugify":   TemplateFuncSlugify,
		"asset":     TemplateFuncAsset,
		"assetMeta": TemplateFuncAssetMeta,
		"dict":      TemplateFuncDict,
		"merge":     TemplateFuncMerge,
		"rawHTML":   TemplateFuncRawHTML,
		"first":     TemplateFuncFirst,
		"asPages":   TemplateFuncAsPages,
		"asPage":    TemplateFuncAsPage,
	}
}

func TemplateFuncDiscard() (string, error) {
	return "", errutil.Discard()
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

func TemplateFuncAsset(key string, page Page) string {
	if asset := TemplateFuncAssetMeta(key, page); asset != nil {
		return asset.URL
	}
	return ""
}

func TemplateFuncAssetMeta(key string, page Page) *PageAsset {
	key = strings.TrimPrefix(path.Clean(strings.TrimSpace(key)), "/")
	if key == "." || key == "" {
		return nil
	}
	return page.Assets[key]
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

func TemplateFuncRawHTML(value string) template.HTML {
	return template.HTML(value)
}

func TemplateFuncFirst(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case *QueryResult:
		if v == nil || len(v.Rows) == 0 {
			return nil
		}
		return v.Rows[0]
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	case []*Page:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	}

	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil
	}
	if rv.Len() == 0 {
		return nil
	}
	return rv.Index(0).Interface()
}

func TemplateFuncAsPages(value any) ([]*Page, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []*Page:
		return v, nil
	case *QueryResult:
		if v == nil {
			return nil, nil
		}
		if v.Pages == nil {
			return nil, fmt.Errorf("asPages requires the result to include the _page column")
		}
		return v.Pages, nil
	default:
		return nil, fmt.Errorf("asPages requires a QueryResult or []*Page, got %T", value)
	}
}

func TemplateFuncAsPage(value any) (*Page, error) {
	pages, err := TemplateFuncAsPages(value)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}
	return pages[0], nil
}

func isTemplateEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []string:
		return len(v) == 0
	case []*Page:
		return len(v) == 0
	case []map[string]any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	case *QueryResult:
		return v == nil || len(v.Rows) == 0
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
