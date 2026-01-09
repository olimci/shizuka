package manifest

import (
	"html/template"
	"io"
	"os"
)

func StaticArtefact(claim Claim) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			file, err := os.Open(claim.Source)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(w, file)
			return err
		},
	}
}

func TemplateArtefact(claim Claim, tmpl *template.Template, data any) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			return tmpl.Execute(w, data)
		},
	}
}

func NamedTemplateArtefact(claim Claim, name string, tmpl *template.Template, data any) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			return tmpl.ExecuteTemplate(w, name, data)
		},
	}
}

func TextArtefact(claim Claim, text string) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			_, err := w.Write([]byte(text))
			return err
		},
	}
}
