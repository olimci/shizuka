package tmplutil

import (
	"fmt"
	"html/template"
	"maps"
	"reflect"
	"time"
)

func DefaultFuncs() template.FuncMap {
	return template.FuncMap{
		"discard":    discard,
		"datefmt":    datefmt,
		"uniq":       unique,
		"dict":       dict,
		"merge":      merge,
		"raw":        raw,
		"first":      first,
		"debug":      debug,
		"debugShort": debugShort,
	}
}

func discard() (string, error) {
	return "", Discard()
}

// datefmt formats a time using Go's time layout.
func datefmt(layout string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(layout)
}

// unique deduplicates strings while preserving order.
func unique(values []string) []string {
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

// dict constructs a map from key/value pairs.
func dict(values ...any) (map[string]any, error) {
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

// merge merges maps left-to-right.
func merge(mapsToMerge ...map[string]any) map[string]any {
	out := make(map[string]any)
	for _, m := range mapsToMerge {
		maps.Copy(out, m)
	}
	return out
}

func raw(value string) template.HTML {
	return template.HTML(value)
}

func first(value any) any {
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
