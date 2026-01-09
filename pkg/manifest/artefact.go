package manifest

import (
	"io"
)

// ArtefactBuilder is a function that builds an artefact
type ArtefactBuilder func(w io.Writer) error

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
