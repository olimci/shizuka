package build

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/olimci/shizuka/pkg/utils/set"
)

func lookupErrPage(pageErrTemplates map[error]*template.Template, err error) *template.Template {
	if err == nil || pageErrTemplates == nil {
		return nil
	}

	for match, tmpl := range pageErrTemplates {
		if match == nil {
			continue
		}

		if errors.Is(err, match) {
			return tmpl
		}
	}

	return pageErrTemplates[nil] // deeply cursed but i like it
}

func parseTemplates(pattern string, funcs template.FuncMap) (*template.Template, error) {
	files, err := doublestar.Glob(os.DirFS("."), pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no template files found matching pattern: %s", pattern)
	}

	tmpl := template.New("").Funcs(funcs)

	seen := set.New[string]()

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", file, err)
		}

		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

		if seen.HasAdd(name) {
			return nil, fmt.Errorf("duplicate template name: %s", name)
		}

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", file, err)
		}
	}

	return tmpl, nil
}
