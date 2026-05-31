package manifest

import (
	"io"
	"io/fs"
)

// ArtefactBuilder writes an artefact's bytes.
type ArtefactBuilder func(w io.Writer) error

// PostProcessor wraps an artefact builder, usually for output transforms.
type PostProcessor func(claim Claim, next ArtefactBuilder) ArtefactBuilder

// Artefact represents one generated output file.
type Artefact struct {
	Claim   Claim
	Builder ArtefactBuilder
}

func (a Artefact) Post(pp PostProcessor) Artefact {
	if pp == nil {
		return a
	}
	a.Builder = pp(a.Claim, a.Builder)
	return a
}

func StaticArtefact(sourceFS fs.FS, claim Claim) Artefact {
	source := claim.Source
	return Artefact{
		Claim: claim,
		Builder: func(w io.Writer) error {
			file, err := sourceFS.Open(source)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(w, file)
			return err
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
