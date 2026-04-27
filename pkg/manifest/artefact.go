package manifest

import (
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/utils/errutil"
)

// ArtefactBuilder is a function that builds an artefact
type ArtefactBuilder = func(w io.Writer) error

type PostProcessor func(claim Claim, next ArtefactBuilder) ArtefactBuilder

// Artefact represents a build artefact
type Artefact struct {
	Claim   Claim
	Builder ArtefactBuilder
}

func (a Artefact) Post(pp PostProcessor) Artefact {
	if pp == nil {
		return a
	}

	return Artefact{
		Claim:   a.Claim,
		Builder: pp(a.Claim, a.Builder),
	}
}

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
				if errutil.IsDiscard(err) {
					return errutil.WrapDiscard(err)
				}
				return err
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
				if errutil.IsDiscard(err) {
					return errutil.WrapDiscard(err)
				}
				return err
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
