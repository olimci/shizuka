package cmd

import (
	"html/template"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/utils/lazy"
)

var templates = lazy.Must(func() (*template.Template, error) {
	return template.New("").
		ParseFS(embed.Templates, "templates/*.tmpl")
})

var (
	templateNotFound = lazy.New(func() *template.Template {
		return templates.Get().Lookup("404")
	})
	templateError = lazy.New(func() *template.Template {
		return templates.Get().Lookup("error")
	})
	templateFallback = lazy.New(func() *template.Template {
		return templates.Get().Lookup("fallback")
	})
	templateBuildError = lazy.New(func() *template.Template {
		return templates.Get().Lookup("build_failed")
	})
)
