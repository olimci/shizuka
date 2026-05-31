package build

import (
	"errors"
	"fmt"
	"html/template"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/olimci/shizuka/internal/transforms"
	"github.com/olimci/shizuka/internal/utils/pathutil"
)

var (
	ErrPaginate        = errors.New("paginate")
	ErrPaginationCycle = errors.New("pagination template cycle")
)

type paginationEffect struct {
	Items        []any
	PerPage      int
	Field        string
	PageTemplate string
	RootTemplate string
}

func (e *paginationEffect) Error() string {
	return ErrPaginate.Error()
}

func (e *paginationEffect) Unwrap() error {
	return ErrPaginate
}

func paginationFuncMap() template.FuncMap {
	return template.FuncMap{
		"paginate":       paginateEffect,
		"paginateRoot":   paginateRootEffect,
		"paginateOn":     paginateOnEffect,
		"paginateOnRoot": paginateOnRootEffect,
	}
}

func paginateEffect(perPage int, templateName string, items any) (string, error) {
	return "", newPaginateEffect(perPage, templateName, "", items)
}

func paginateRootEffect(perPage int, pageTemplate, rootTemplate string, items any) (string, error) {
	return "", newPaginateEffect(perPage, pageTemplate, rootTemplate, items)
}

func paginateOnEffect(field, templateName string, items any) (string, error) {
	return "", newPaginateOnEffect(field, templateName, "", items)
}

func paginateOnRootEffect(field, pageTemplate, rootTemplate string, items any) (string, error) {
	return "", newPaginateOnEffect(field, pageTemplate, rootTemplate, items)
}

func newPaginateEffect(perPage int, pageTemplate, rootTemplate string, items any) error {
	if perPage <= 0 {
		return fmt.Errorf("paginate perPage must be greater than zero")
	}
	if strings.TrimSpace(pageTemplate) == "" {
		return fmt.Errorf("paginate page template is empty")
	}
	if rootTemplate != "" && strings.TrimSpace(rootTemplate) == "" {
		return fmt.Errorf("paginate root template is empty")
	}
	slice, err := sliceAny(items)
	if err != nil {
		return fmt.Errorf("paginate items: %w", err)
	}
	return &paginationEffect{
		Items:        slice,
		PerPage:      perPage,
		PageTemplate: pageTemplate,
		RootTemplate: rootTemplate,
	}
}

func newPaginateOnEffect(field, pageTemplate, rootTemplate string, items any) error {
	if strings.TrimSpace(field) == "" {
		return fmt.Errorf("paginateOn field is empty")
	}
	if strings.TrimSpace(pageTemplate) == "" {
		return fmt.Errorf("paginateOn page template is empty")
	}
	if rootTemplate != "" && strings.TrimSpace(rootTemplate) == "" {
		return fmt.Errorf("paginateOn root template is empty")
	}
	slice, err := sliceAny(items)
	if err != nil {
		return fmt.Errorf("paginateOn items: %w", err)
	}
	return &paginationEffect{
		Items:        slice,
		Field:        field,
		PageTemplate: pageTemplate,
		RootTemplate: rootTemplate,
	}
}

func sliceAny(value any) ([]any, error) {
	if value == nil {
		return nil, fmt.Errorf("expected slice or array, got nil")
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("expected slice or array, got nil pointer")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, fmt.Errorf("expected slice or array, got %T", value)
	}

	out := make([]any, rv.Len())
	for i := range rv.Len() {
		out[i] = rv.Index(i).Interface()
	}
	return out, nil
}

type paginationPage struct {
	Route string
	Data  transforms.PaginationTmpl
}

func buildPaginationPages(baseRoute string, effect paginationEffect) ([]paginationPage, error) {
	if effect.Field != "" {
		return buildGroupPaginationPages(baseRoute, effect)
	}
	return buildChunkPaginationPages(baseRoute, effect)
}

func buildChunkPaginationPages(baseRoute string, effect paginationEffect) ([]paginationPage, error) {
	total := len(effect.Items)
	if total == 0 {
		return nil, nil
	}

	pages := (total + effect.PerPage - 1) / effect.PerPage
	out := make([]paginationPage, 0, pages)
	for pageNum := 1; pageNum <= pages; pageNum++ {
		start := (pageNum - 1) * effect.PerPage
		end := min(start+effect.PerPage, total)
		route, err := paginationChildRoute(baseRoute, strconv.Itoa(pageNum))
		if err != nil {
			return nil, err
		}

		var prev, next string
		if pageNum > 1 {
			prev, err = paginationChildRoute(baseRoute, strconv.Itoa(pageNum-1))
			if err != nil {
				return nil, err
			}
		}
		if pageNum < pages {
			next, err = paginationChildRoute(baseRoute, strconv.Itoa(pageNum+1))
			if err != nil {
				return nil, err
			}
		}

		out = append(out, paginationPage{
			Route: route,
			Data: transforms.PaginationTmpl{
				Items:   effect.Items[start:end],
				Page:    pageNum,
				Pages:   pages,
				Total:   total,
				PerPage: effect.PerPage,
				Prev:    prev,
				Next:    next,
			},
		})
	}
	return out, nil
}

func buildGroupPaginationPages(baseRoute string, effect paginationEffect) ([]paginationPage, error) {
	type group struct {
		value any
		items []any
	}

	groups := make([]group, 0)
	index := make(map[any]int)
	for itemIdx, item := range effect.Items {
		value, err := paginationGroupValue(item, effect.Field)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", itemIdx, err)
		}
		if value == nil {
			return nil, fmt.Errorf("item %d: group value %q is nil", itemIdx, effect.Field)
		}
		if !reflect.TypeOf(value).Comparable() {
			return nil, fmt.Errorf("item %d: group value %q is not comparable", itemIdx, effect.Field)
		}
		pos, ok := index[value]
		if !ok {
			pos = len(groups)
			index[value] = pos
			groups = append(groups, group{value: value})
		}
		groups[pos].items = append(groups[pos].items, item)
	}

	out := make([]paginationPage, 0, len(groups))
	for _, group := range groups {
		segment := fmt.Sprint(group.value)
		route, err := paginationChildRoute(baseRoute, segment)
		if err != nil {
			return nil, fmt.Errorf("group %q: %w", segment, err)
		}
		out = append(out, paginationPage{
			Route: route,
			Data: transforms.PaginationTmpl{
				Items:   group.items,
				Page:    1,
				Pages:   1,
				Total:   len(group.items),
				PerPage: len(group.items),
				Group:   group.value,
			},
		})
	}
	return out, nil
}

func paginationGroupValue(item any, field string) (any, error) {
	rv := reflect.ValueOf(item)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, fmt.Errorf("cannot read %q from nil", field)
		}
		rv = rv.Elem()
	}

	if rv.Kind() == reflect.Map {
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("cannot read %q from map with %s keys", field, rv.Type().Key())
		}
		value := rv.MapIndex(reflect.ValueOf(field))
		if !value.IsValid() {
			return nil, fmt.Errorf("missing group field %q", field)
		}
		return value.Interface(), nil
	}

	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cannot read %q from %T", field, item)
	}
	value := rv.FieldByName(field)
	if !value.IsValid() {
		return nil, fmt.Errorf("missing group field %q", field)
	}
	if !value.CanInterface() {
		return nil, fmt.Errorf("group field %q is not exported", field)
	}
	return value.Interface(), nil
}

func paginationChildRoute(baseRoute, segment string) (string, error) {
	if segment == "" {
		return "", fmt.Errorf("path segment is empty")
	}
	if strings.Contains(segment, "/") {
		return "", fmt.Errorf("path segment must not contain /")
	}
	baseRoute, err := pathutil.ValidateRoutePath(baseRoute)
	if err != nil {
		return "", err
	}
	route := "/" + path.Join(strings.Trim(baseRoute, "/"), segment) + "/"
	if baseRoute == "/" {
		route = "/" + segment + "/"
	}
	return pathutil.ValidateRoutePath(route)
}
