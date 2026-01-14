package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/utils/set"
	"github.com/tdewolff/minify/v2"
	mincss "github.com/tdewolff/minify/v2/css"
	minhtml "github.com/tdewolff/minify/v2/html"
	minjs "github.com/tdewolff/minify/v2/js"
)

func NewMinifier(enabled bool) manifest.PostProcessor {
	if !enabled {
		return nil
	}

	mimes := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
	}

	m := minify.New()
	m.AddFunc("text/html", minhtml.Minify)
	m.AddFunc("text/css", mincss.Minify)
	m.AddFunc("application/javascript", minjs.Minify)

	return func(claim manifest.Claim, next manifest.ArtefactBuilder) manifest.ArtefactBuilder {
		mime, ex := mimes[filepath.Ext(claim.Target)]
		if !ex {
			return next
		}

		return func(w io.Writer) error {
			x := m.Writer(mime, w)
			if err := next(x); err != nil {
				return err
			}
			return x.Close()
		}
	}
}

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

func url2dir(dir string) string {
	dir = strings.TrimSuffix(dir, string(filepath.Separator))
	dir = filepath.ToSlash(dir)
	if dir == "." {
		return ""
	}
	return dir
}

func shortSlugForRedirect(slug string) string {
	slug = strings.TrimSpace(slug)
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return ""
	}
	if i := strings.LastIndex(slug, "/"); i >= 0 && i < len(slug)-1 {
		return slug[i+1:]
	}
	return slug
}
