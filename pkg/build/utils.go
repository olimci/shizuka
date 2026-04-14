package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
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

func parseTemplates(sourceRoot, pattern string, funcs template.FuncMap) (*template.Template, error) {
	files, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("template glob %q: %w", pattern, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no templates matched %q", pattern)
	}

	tmpl := template.New("shizuka").Funcs(funcs)

	seen := set.New[string]()

	for _, file := range files {
		rel, err := filepath.Rel(sourceRoot, file)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", file, err)
		}
		rel = filepath.ToSlash(rel)

		content, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", rel, err)
		}

		if seen.HasAdd(rel) {
			return nil, fmt.Errorf("template %q was matched more than once", rel)
		}

		_, err = tmpl.New(rel).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", rel, err)
		}
	}

	return tmpl, nil
}

var (
	cleanFSPath = pathutil.CleanContentPath
	cleanFSGlob = pathutil.CleanContentGlob
)
