package tmplutil

import (
	"fmt"
	"html"
	"html/template"
	"reflect"
	"sort"
	"strings"
	"time"
)

var (
	errorType = reflect.TypeFor[error]()
	htmlType  = reflect.TypeFor[template.HTML]()
	timeType  = reflect.TypeFor[time.Time]()
)

type debugOptions struct {
	omitEmpty bool
	maxDepth  int
}

type debugRenderer struct {
	options debugOptions
	seen    map[visit]struct{}
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}

func debug(value any) template.HTML {
	return renderDebug(value, debugOptions{maxDepth: 16})
}

func debugShort(value any) template.HTML {
	return renderDebug(value, debugOptions{omitEmpty: true, maxDepth: 16})
}

func renderDebug(value any, options debugOptions) template.HTML {
	r := debugRenderer{
		options: options,
		seen:    make(map[visit]struct{}),
	}
	return template.HTML(r.renderValue(reflect.ValueOf(value), 0))
}

func (r *debugRenderer) renderValue(value reflect.Value, depth int) string {
	if !value.IsValid() {
		return r.renderMarker("nil", "nil")
	}

	if depth > r.options.maxDepth {
		return r.renderMarker("max-depth", "max depth reached")
	}

	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		return r.renderValue(value.Elem(), depth)
	}

	if err, ok := reflectError(value); ok {
		if isNilable(value.Kind()) && value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		if hasExportedStructFields(value) {
			return r.renderErrorStruct(value, err, depth)
		}
		return r.renderError(err)
	}

	if value.CanAddr() && reflect.PointerTo(value.Type()).Implements(errorType) {
		if err, ok := value.Addr().Interface().(error); ok {
			if hasExportedStructFields(value) {
				return r.renderErrorStruct(value, err, depth)
			}
			return r.renderError(err)
		}
	}

	if value.Type() == htmlType {
		return `<div class="shizuka-debug-html shizuka-debug-kind-html">` + string(value.Interface().(template.HTML)) + `</div>`
	}

	if value.Type() == timeType {
		t := value.Interface().(time.Time)
		if t.IsZero() {
			return r.renderScalar("time", "")
		}
		return r.renderScalar("time", t.Format(time.RFC3339))
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		key := visit{typ: value.Type(), ptr: value.Pointer()}
		if _, ok := r.seen[key]; ok {
			return r.renderMarker("cycle", "cycle")
		}
		r.seen[key] = struct{}{}
		out := r.renderValue(value.Elem(), depth+1)
		delete(r.seen, key)
		return out
	case reflect.Struct:
		return r.renderStruct(value, depth)
	case reflect.Map:
		if value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		key := visit{typ: value.Type(), ptr: value.Pointer()}
		if _, ok := r.seen[key]; ok {
			return r.renderMarker("cycle", "cycle")
		}
		r.seen[key] = struct{}{}
		out := r.renderMap(value, depth)
		delete(r.seen, key)
		return out
	case reflect.Slice:
		if value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		if value.Len() > 0 {
			key := visit{typ: value.Type(), ptr: value.Pointer()}
			if _, ok := r.seen[key]; ok {
				return r.renderMarker("cycle", "cycle")
			}
			r.seen[key] = struct{}{}
			out := r.renderSequence(value, depth, "slice")
			delete(r.seen, key)
			return out
		}
		return r.renderSequence(value, depth, "slice")
	case reflect.Array:
		return r.renderSequence(value, depth, "array")
	case reflect.String:
		return r.renderScalar("string", value.String())
	case reflect.Bool:
		return r.renderScalar("bool", fmt.Sprint(value.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return r.renderScalar("int", fmt.Sprint(value.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return r.renderScalar("uint", fmt.Sprint(value.Uint()))
	case reflect.Float32, reflect.Float64:
		return r.renderScalar("float", fmt.Sprint(value.Float()))
	case reflect.Complex64, reflect.Complex128:
		return r.renderScalar("complex", fmt.Sprint(value.Complex()))
	case reflect.Invalid:
		return r.renderMarker("nil", "nil")
	default:
		if value.CanInterface() {
			return r.renderScalar(safeClassPart(value.Kind().String()), fmt.Sprint(value.Interface()))
		}
		return r.renderMarker("unreadable", "unreadable")
	}
}

func (r *debugRenderer) renderStruct(value reflect.Value, depth int) string {
	typ := value.Type()
	rows := make([]debugRow, 0, value.NumField())
	for i := range value.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldValue := value.Field(i)
		if r.options.omitEmpty && isDebugEmpty(fieldValue) {
			continue
		}
		rows = append(rows, debugRow{
			key:      field.Name,
			keyClass: "shizuka-debug-field-" + safeClassPart(field.Name),
			value:    r.renderValue(fieldValue, depth+1),
		})
	}
	return renderDebugTable("struct", typeClass(typ), rows)
}

func (r *debugRenderer) renderErrorStruct(value reflect.Value, err error, depth int) string {
	if depth > r.options.maxDepth {
		return r.renderMarker("max-depth", "max depth reached")
	}

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return r.renderMarker("nil", "nil")
		}
		key := visit{typ: value.Type(), ptr: value.Pointer()}
		if _, ok := r.seen[key]; ok {
			return r.renderMarker("cycle", "cycle")
		}
		r.seen[key] = struct{}{}
		out := r.renderErrorStruct(value.Elem(), err, depth+1)
		delete(r.seen, key)
		return out
	}

	if value.Kind() != reflect.Struct {
		return r.renderError(err)
	}

	typ := value.Type()
	rows := []debugRow{
		{
			key:      "Error",
			keyClass: "shizuka-debug-field-error",
			value:    r.renderError(err),
		},
	}
	for i := range value.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldValue := value.Field(i)
		if r.options.omitEmpty && isDebugEmpty(fieldValue) {
			continue
		}
		rows = append(rows, debugRow{
			key:      field.Name,
			keyClass: "shizuka-debug-field-" + safeClassPart(field.Name),
			value:    r.renderValue(fieldValue, depth+1),
		})
	}
	return renderDebugTable("error", typeClass(typ), rows)
}

func (r *debugRenderer) renderMap(value reflect.Value, depth int) string {
	iter := value.MapRange()
	rows := make([]debugRow, 0, value.Len())
	for iter.Next() {
		mapValue := iter.Value()
		if r.options.omitEmpty && isDebugEmpty(mapValue) {
			continue
		}
		key := stringifyReflectValue(iter.Key())
		rows = append(rows, debugRow{
			key:      key,
			keyClass: "shizuka-debug-map-key-" + safeClassPart(key),
			value:    r.renderValue(mapValue, depth+1),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].key < rows[j].key
	})
	return renderDebugTable("map", typeClass(value.Type()), rows)
}

func (r *debugRenderer) renderSequence(value reflect.Value, depth int, kind string) string {
	rows := make([]debugRow, 0, value.Len())
	for i := range value.Len() {
		item := value.Index(i)
		if r.options.omitEmpty && isDebugEmpty(item) {
			continue
		}
		key := fmt.Sprintf("%d", i)
		rows = append(rows, debugRow{
			key:      key,
			keyClass: "shizuka-debug-index",
			value:    r.renderValue(item, depth+1),
		})
	}
	return renderDebugTable(kind, typeClass(value.Type()), rows)
}

func (r *debugRenderer) renderScalar(kind, value string) string {
	class := "shizuka-debug-scalar shizuka-debug-kind-" + safeClassPart(kind)
	return `<span class="` + class + `">` + html.EscapeString(value) + `</span>`
}

func (r *debugRenderer) renderError(err error) string {
	return `<span class="shizuka-debug-error shizuka-debug-kind-error">` + html.EscapeString(err.Error()) + `</span>`
}

func (r *debugRenderer) renderMarker(kind, value string) string {
	class := "shizuka-debug-marker shizuka-debug-" + safeClassPart(kind)
	return `<span class="` + class + `">` + html.EscapeString(value) + `</span>`
}

type debugRow struct {
	key      string
	keyClass string
	value    string
}

func renderDebugTable(kind, typ string, rows []debugRow) string {
	classes := []string{"shizuka-debug", "shizuka-debug-kind-" + safeClassPart(kind)}
	if typ != "" {
		classes = append(classes, typ)
	}

	var b strings.Builder
	b.WriteString(`<table class="`)
	b.WriteString(strings.Join(classes, " "))
	b.WriteString(`"><tbody>`)
	for _, row := range rows {
		b.WriteString(`<tr class="shizuka-debug-row`)
		if row.keyClass != "" {
			b.WriteByte(' ')
			b.WriteString(row.keyClass)
		}
		b.WriteString(`"><th class="shizuka-debug-key">`)
		b.WriteString(html.EscapeString(row.key))
		b.WriteString(`</th><td class="shizuka-debug-value">`)
		b.WriteString(row.value)
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

func stringifyReflectValue(value reflect.Value) string {
	if !value.IsValid() {
		return "nil"
	}
	if value.CanInterface() {
		return fmt.Sprint(value.Interface())
	}
	return "unreadable"
}

func isDebugEmpty(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return true
		}
		value = value.Elem()
	}
	return value.IsZero()
}

func isNilable(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

func reflectError(value reflect.Value) (error, bool) {
	if !value.IsValid() {
		return nil, false
	}
	if !value.CanInterface() {
		return nil, false
	}
	if !value.Type().Implements(errorType) {
		return nil, false
	}
	if isNilable(value.Kind()) && value.IsNil() {
		return nil, true
	}
	err, ok := value.Interface().(error)
	return err, ok
}

func hasExportedStructFields(value reflect.Value) bool {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return false
	}
	typ := value.Type()
	for i := range value.NumField() {
		if typ.Field(i).IsExported() {
			return true
		}
	}
	return false
}

func typeClass(typ reflect.Type) string {
	if typ == nil {
		return ""
	}
	name := typ.String()
	if name == "" {
		return ""
	}
	return "shizuka-debug-type-" + safeClassPart(name)
}

func safeClassPart(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "value"
	}
	return out
}
