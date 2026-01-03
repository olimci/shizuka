package cmd

import (
	"fmt"
	"html/template"

	"github.com/olimci/shizuka/cmd/embed"
)

func load404Template() (*template.Template, error) {
	tmpl, err := template.New("404").ParseFS(embed.Templates, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse 404 templates: %w", err)
	}

	notFound := tmpl.Lookup("404")
	if notFound == nil {
		return nil, fmt.Errorf("404 template missing")
	}

	return notFound, nil
}
