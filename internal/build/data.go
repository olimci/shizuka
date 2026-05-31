package build

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/olimci/shizuka/internal/utils/decodeutil"
	"github.com/olimci/structql"
)

type dataTable struct {
	Name   string
	Rows   []map[string]any
	Source string
}

type dataManifest struct {
	Tables map[string]dataManifestTable `toml:"tables" yaml:"tables" json:"tables"`
}

type dataManifestTable struct {
	Name string `toml:"name" yaml:"name" json:"name"`
	Rows any    `toml:"rows" yaml:"rows" json:"rows"`
}

func loadDataTables(source fs.FS, dataPath string) ([]dataTable, error) {
	info, err := fs.Stat(source, dataPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("data source %q: %w", dataPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("data source %q is not a directory", dataPath)
	}

	var files []string
	if err := fs.WalkDir(source, dataPath, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, ok := decodeutil.FormatExt(path.Ext(filePath)); ok {
			files = append(files, filePath)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("data source %q: %w", dataPath, err)
	}
	slices.Sort(files)

	var tables []dataTable
	for _, filePath := range files {
		fileTables, err := loadDataManifest(source, filePath)
		if err != nil {
			return nil, err
		}
		tables = append(tables, fileTables...)
	}
	return tables, nil
}

func loadDataManifest(source fs.FS, filePath string) ([]dataTable, error) {
	doc, err := fs.ReadFile(source, filePath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}

	var manifest dataManifest
	if err := decodeutil.UnmarshalExt(path.Ext(filePath), doc, &manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", filePath, err)
	}
	if len(manifest.Tables) == 0 {
		return nil, fmt.Errorf("%s: data manifest must define at least one table", filePath)
	}

	keys := make([]string, 0, len(manifest.Tables))
	for key := range manifest.Tables {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	tables := make([]dataTable, 0, len(keys))
	for _, key := range keys {
		spec := manifest.Tables[key]
		if !validTableName(spec.Name) {
			return nil, fmt.Errorf("%s: tables.%s.name %q is not a valid table identifier", filePath, key, spec.Name)
		}
		rows, err := dataRows(spec.Rows)
		if err != nil {
			return nil, fmt.Errorf("%s: tables.%s.rows: %w", filePath, key, err)
		}
		tables = append(tables, dataTable{Name: spec.Name, Rows: rows, Source: filePath})
	}
	return tables, nil
}

func registerDataTables(db *structql.DB, tables []dataTable) error {
	for _, data := range tables {
		table, err := structql.BuildMapTable(data.Rows)
		if err != nil {
			return fmt.Errorf("%s: build table %q: %w", data.Source, data.Name, err)
		}
		if err := db.Register(data.Name, table); err != nil {
			return fmt.Errorf("%s: register table %q: %w", data.Source, data.Name, err)
		}
	}
	return nil
}

func dataRows(value any) ([]map[string]any, error) {
	if value == nil {
		return nil, fmt.Errorf("missing rows")
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("missing rows")
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		rows := make([]map[string]any, 0, rv.Len())
		for i := range rv.Len() {
			row, err := dataObject(rv.Index(i).Interface())
			if err != nil {
				return nil, fmt.Errorf("row %d: %w", i, err)
			}
			rows = append(rows, row)
		}
		return rows, nil

	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("keyed rows must use string keys")
		}
		keys := rv.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int {
			return strings.Compare(a.String(), b.String())
		})
		rows := make([]map[string]any, 0, len(keys))
		for _, key := range keys {
			row, err := dataObject(rv.MapIndex(key).Interface())
			if err != nil {
				return nil, fmt.Errorf("row %q: %w", key.String(), err)
			}
			row = maps.Clone(row)
			for col := range row {
				if strings.EqualFold(col, "key") {
					return nil, fmt.Errorf("row %q: keyed row must not define reserved column \"key\"", key.String())
				}
			}
			row["key"] = key.String()
			rows = append(rows, row)
		}
		return rows, nil

	default:
		return nil, fmt.Errorf("expected array of objects or keyed object, got %T", value)
	}
}

func dataObject(value any) (map[string]any, error) {
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, fmt.Errorf("expected object, got nil")
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("expected object, got %T", value)
	}

	row := make(map[string]any, rv.Len())
	iter := rv.MapRange()
	for iter.Next() {
		row[iter.Key().String()] = iter.Value().Interface()
	}
	return row, nil
}

func validTableName(name string) bool {
	if name == "" {
		return false
	}
	if reservedTableName(name) {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func reservedTableName(name string) bool {
	switch strings.ToUpper(name) {
	case "SELECT", "DISTINCT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER",
		"ON", "AS", "ORDER", "GROUP", "BY", "HAVING", "LIMIT", "ASC", "DESC",
		"AND", "OR", "NOT", "IN", "IS", "NULL", "TRUE", "FALSE":
		return true
	default:
		return false
	}
}
