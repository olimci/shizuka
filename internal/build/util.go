package build

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/olimci/shizuka/internal/build/embed"
	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/manifest"
	"github.com/olimci/shizuka/internal/transforms"
	"github.com/olimci/shizuka/internal/utils/tmplutil"
	"github.com/tdewolff/minify/v2"
	mincss "github.com/tdewolff/minify/v2/css"
	minhtml "github.com/tdewolff/minify/v2/html"
	minjs "github.com/tdewolff/minify/v2/js"
)

func buildLogger(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return logger.With("component", "build")
}

func componentLogger(logger *slog.Logger, component string) *slog.Logger {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return logger.With("component", component)
}

func NewMinifier(cfg *config.ConfigMinifier) manifest.PostProcessor {
	if cfg == nil {
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

		target := strings.TrimPrefix(filepath.ToSlash(path.Clean(claim.Target)), "/")
		if target == "." || target == "" {
			return next
		}
		included := len(cfg.Whitelist) == 0 || matchAnyPattern(cfg.Whitelist, target)
		excluded := matchAnyPattern(cfg.Blacklist, target)
		if !included || excluded {
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

func matchAnyPattern(patterns []string, target string) bool {
	for _, pattern := range patterns {
		if matchGitignorePattern(pattern, target) {
			return true
		}
	}
	return false
}

func matchGitignorePattern(pattern, target string) bool {
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")

	if before, ok := strings.CutSuffix(pattern, "/"); ok {
		prefix := before
		if anchored {
			return target == prefix || strings.HasPrefix(target, prefix+"/")
		}
		return target == prefix || strings.HasPrefix(target, prefix+"/") || strings.Contains(target, "/"+prefix+"/")
	}

	if !strings.Contains(pattern, "/") {
		return doublestar.MatchUnvalidated(pattern, path.Base(target))
	}
	if anchored {
		return doublestar.MatchUnvalidated(pattern, target)
	}
	return doublestar.MatchUnvalidated(pattern, target) || doublestar.MatchUnvalidated("**/"+pattern, target)
}

func parseRequiredTemplates(sourceFS fs.FS, pattern string, funcs template.FuncMap) (*template.Template, error) {
	files, err := doublestar.Glob(sourceFS, pattern, doublestar.WithFailOnIOErrors())
	if err != nil {
		return nil, fmt.Errorf("template glob %q: %w", pattern, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no templates matched %q", pattern)
	}
	return parseTemplateFiles(sourceFS, files, funcs)
}

func parseOptionalTemplates(sourceFS fs.FS, pattern string, funcs template.FuncMap) (*template.Template, error) {
	files, err := doublestar.Glob(sourceFS, pattern, doublestar.WithFailOnIOErrors())
	if err != nil {
		return nil, fmt.Errorf("template glob %q: %w", pattern, err)
	}
	if len(files) == 0 {
		return template.New("shizuka").Funcs(funcs), nil
	}
	return parseTemplateFiles(sourceFS, files, funcs)
}

func parseTemplateFiles(sourceFS fs.FS, files []string, funcs template.FuncMap) (*template.Template, error) {
	tmpl := template.New("shizuka").Funcs(funcs)

	seen := make(map[string]struct{}, len(files))

	for _, file := range files {
		rel := path.Clean(file)
		content, err := fs.ReadFile(sourceFS, rel)
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", rel, err)
		}

		if _, ok := seen[rel]; ok {
			return nil, fmt.Errorf("template %q was matched more than once", rel)
		}
		seen[rel] = struct{}{}

		_, err = tmpl.New(rel).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("template %q: %w", rel, err)
		}
	}

	return tmpl, nil
}

func renderMarkdownComponentTemplate(tmpl *template.Template, page *transforms.Page) (string, error) {
	tmpl, err := tmpl.Clone()
	if err != nil {
		return "", err
	}
	if _, err := tmpl.New(page.SourcePath).Parse(page.RawBody); err != nil {
		return "", fmt.Errorf("markdown template %q: %w", page.SourcePath, err)
	}

	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, page.SourcePath, page.Tmpl()); err != nil {
		return "", fmt.Errorf("markdown template %q: %w", page.SourcePath, err)
	}
	return buf.String(), nil
}

func emitDebugTemplate(sc *StepContext, claim manifest.Claim, data any, pp manifest.PostProcessor) error {
	var buf strings.Builder
	err := embed.Templates.Get().ExecuteTemplate(&buf, "debug", data)
	if tmplutil.IsDiscard(err) {
		return nil
	}
	if err != nil {
		sc.Error(err, claim)
		return nil
	}
	return sc.Manifest.Emit(manifest.TextArtefact(claim, buf.String()).Post(pp))
}

func emitRenderedTemplate(sc *StepContext, claim manifest.Claim, tmpl *template.Template, name string, data any, pp manifest.PostProcessor) error {
	var buf strings.Builder
	err := tmpl.ExecuteTemplate(&buf, name, data)
	if tmplutil.IsDiscard(err) {
		return nil
	}
	if err != nil {
		sc.Error(err, claim)
		return nil
	}
	return sc.Manifest.Emit(manifest.TextArtefact(claim, buf.String()).Post(pp))
}
