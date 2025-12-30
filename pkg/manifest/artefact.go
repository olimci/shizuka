package manifest

import (
	"io"
)

type Claim struct {
	Owner  string
	Source string
	Target string

	Tags []string
}

func (c Claim) AddTag(tag string) Claim {
	c.Tags = append(c.Tags, tag)
	return c
}

type ArtefactBuilder func(w io.Writer) error

type Artefact struct {
	Claim   Claim
	Builder ArtefactBuilder
}
