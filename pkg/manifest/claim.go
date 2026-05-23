package manifest

import (
	"fmt"
	"path"
	"strings"
)

func NewPageClaim(source, routePath string) Claim {
	targetPath := strings.Trim(routePath, "/")
	target := "index.html"
	if targetPath != "" {
		target = path.Join(targetPath, "index.html")
	}
	return Claim{
		Source: source,
		Target: target,
		Canon:  routePath,
	}
}

func NewInternalClaim(owner, target string) Claim {
	return Claim{
		Owner:  owner,
		Target: target,
		Canon:  target,
	}
}

// Claim represents an artefact's claim on a target path
type Claim struct {
	Owner string

	Source string
	Target string
	Canon  string
}

func (c Claim) Own(name string) Claim {
	c.Owner = name
	return c
}

func (c Claim) DisplayOwner() string {
	if c.Owner != "" {
		return c.Owner
	}
	if c.Source != "" {
		return c.Source
	}
	if c.Target != "" {
		return c.Target
	}
	return "<unknown>"
}

func (c Claim) String() string {
	return fmt.Sprintf("Claim{Owner: %s, Source: %s, Target: %s, Canon: %s}", c.Owner, c.Source, c.Target, c.Canon)
}
