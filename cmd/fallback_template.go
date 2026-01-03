package cmd

import (
	"fmt"
	"html/template"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/transforms"
)

var fallbackTemplateFiles = []string{
	"templates/err_style.tmpl",
	"templates/fallback.tmpl",
}

func loadFallbackTemplate() (*template.Template, error) {
	tmpl, err := template.New("fallback").Funcs(template.FuncMap{
		"where": transforms.TemplateFuncWhere,
		"sort":  transforms.TemplateFuncSortBy,
		"limit": transforms.TemplateFuncLimit,
	}).ParseFS(embed.Templates, "templates/*.tmpl")

	if err != nil {
		return nil, fmt.Errorf("parse fallback templates: %w", err)
	}

	fallback := tmpl.Lookup("fallback")
	if fallback == nil {
		return nil, fmt.Errorf("fallback template missing")
	}

	return fallback, nil
}
