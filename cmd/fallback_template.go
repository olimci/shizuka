package cmd

import (
	"fmt"
	"html/template"

	"github.com/olimci/shizuka/cmd/embed"
	"github.com/olimci/shizuka/pkg/transforms"
)

func loadDevErrTemplates() (*template.Template, *template.Template, *template.Template, error) {
	tmpl, err := template.New("shizuka-errors").
		Funcs(transforms.DefaultTemplateFuncs()).
		ParseFS(embed.Templates, "templates/*.tmpl")

	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse fallback templates: %w", err)
	}

	fallback := tmpl.Lookup("fallback")
	if fallback == nil {
		return nil, nil, nil, fmt.Errorf("fallback template missing")
	}

	errPage := tmpl.Lookup("error")
	if errPage == nil {
		return nil, nil, nil, fmt.Errorf("error template missing")
	}

	buildFailed := tmpl.Lookup("build_failed")
	if buildFailed == nil {
		return nil, nil, nil, fmt.Errorf("build_failed template missing")
	}

	return fallback, errPage, buildFailed, nil
}
