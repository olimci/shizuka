package embed

import (
	"html"
	"html/template"

	"github.com/olimci/shizuka/pkg/utils/lazy"
	"github.com/olimci/shizuka/pkg/utils/tmplutil"
	"github.com/olimci/shizuka/pkg/version"
)

var Templates = lazy.Must(func() (*template.Template, error) {
	funcs := tmplutil.DefaultFuncs()
	funcs["shizukaBanner"] = shizukaBanner

	return template.New("").
		Funcs(funcs).
		ParseFS(templates, "templates/*.tmpl")
})

func shizukaBanner() template.HTML {
	const repoURL = "https://github.com/olimci/shizuka"
	return template.HTML(
		`░█▀▀░█░█░▀█▀░▀▀█░█░█░█░█░█▀█ v` + html.EscapeString(version.String()) + "\n" +
			`░▀▀█░█▀█░░█░░▄▀░░█░█░█▀▄░█▀█` + "\n" +
			`░▀▀▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀░▀░▀░▀ ` + `<a href="` + html.EscapeString(repoURL) + `">` + html.EscapeString(repoURL) + `</a>`,
	)
}

var TemplateDebug = lazy.New(func() *template.Template {
	return Templates.Get().Lookup("debug")
})
