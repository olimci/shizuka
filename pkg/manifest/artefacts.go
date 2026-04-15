package manifest

import (
	"errors"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/transforms"
)

func StaticArtefact(sourceRoot string, claim Claim) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			file, err := os.Open(filepath.Join(sourceRoot, filepath.FromSlash(claim.Source)))
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
			if err := tmpl.Execute(w, data); err != nil {
				return wrapTemplateControlError(err)
			}
			return nil
		},
	}
}

func NamedTemplateArtefact(claim Claim, name string, tmpl *template.Template, data any) Artefact {
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
				return wrapTemplateControlError(err)
			}
			return nil
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

func wrapTemplateControlError(err error) error {
	if err == nil {
		return nil
	}
	if errors.As(err, new(*discardError)) {
		return err
	}
	if transforms.IsDiscardError(err) {
		return &discardError{Cause: err}
	}
	return err
}
