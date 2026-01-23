package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
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

func parseTemplates(fsys fs.FS, pattern string, funcs template.FuncMap) (*template.Template, error) {
	if fsys == nil {
		fsys = os.DirFS(".")
	}
	files, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no template files found matching pattern: %s", pattern)
	}

	tmpl := template.New("").Funcs(funcs)

	seen := set.New[string]()

	for _, file := range files {
		content, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", file, err)
		}

		name := strings.TrimSuffix(path.Base(file), path.Ext(file))

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
	dir = filepath.ToSlash(dir)
	dir = strings.TrimSuffix(dir, "/")
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

func ensureLeadingSlash(target string) string {
	if target == "" {
		return "/"
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return "/" + target
}

func cleanFSPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute paths are not supported: %q", p)
	}
	p = filepath.ToSlash(p)
	p = path.Clean(p)
	if p == "." {
		return ".", nil
	}
	if strings.HasPrefix(p, "../") || p == ".." {
		return "", fmt.Errorf("path %q escapes source root", p)
	}
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ".", nil
	}
	return p, nil
}

func cleanFSGlob(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", fmt.Errorf("empty glob")
	}
	if filepath.IsAbs(pattern) {
		return "", fmt.Errorf("absolute globs are not supported: %q", pattern)
	}
	pattern = filepath.ToSlash(pattern)
	if strings.HasPrefix(pattern, "../") || pattern == ".." {
		return "", fmt.Errorf("glob %q escapes source root", pattern)
	}
	pattern = strings.TrimPrefix(pattern, "/")
	if pattern == "" {
		return "", fmt.Errorf("empty glob")
	}
	return pattern, nil
}
