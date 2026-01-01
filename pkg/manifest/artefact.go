package manifest

import (
	"io"
)

// Claim represents an artefact's claim on a target path
type Claim struct {
	Owner  string
	Source string
	Target string

	Tags []string
}

// AddTag adds a tag to the claim, used for debugging
func (c Claim) AddTag(tag string) Claim {
	c.Tags = append(c.Tags, tag)
	return c
}

// ArtefactBuilder is a function that builds an artefact
type ArtefactBuilder func(w io.Writer) error

// Artefact represents a build artefact
type Artefact struct {
	Claim   Claim
	Builder ArtefactBuilder
}
