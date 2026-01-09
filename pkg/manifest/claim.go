package manifest

import (
	"fmt"
	"path/filepath"
	"strings"
)

func NewPageClaim(sourceRoot, targetRoot, rel string) Claim {
	dir, base := filepath.Split(rel)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	if name == "index" {
		return Claim{
			Source: filepath.Join(sourceRoot, rel),
			Target: filepath.Join(targetRoot, dir, "index.html"),
			Canon:  filepath.Join(targetRoot, dir),
		}
	} else {
		return Claim{
			Source: filepath.Join(sourceRoot, rel),
			Target: filepath.Join(targetRoot, dir, name, "index.html"),
			Canon:  filepath.Join(targetRoot, dir, name),
		}
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
