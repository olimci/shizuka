package manifest

import (
	"fmt"
	"path"
)

func NewPageClaim(source, url string) Claim {
	target := path.Join(url, "index.html")
	return Claim{
		Source: source,
		Target: target,
		Canon:  url,
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

func (c Claim) String() string {
	return fmt.Sprintf("Claim{Owner: %s, Source: %s, Target: %s, Canon: %s}", c.Owner, c.Source, c.Target, c.Canon)
}
