package manifest

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/utils/set"
)

// isRel checks if a path is relative
func isRel(p string) bool {
	for {
		if p == ".." {
			return true
		}
		dir, base := path.Split(p)
		if base == ".." {
			return true
		}
		if dir == "" || dir == "/" {
			return false
		}
		p = strings.TrimSuffix(dir, "/")
	}
}

// manifestDirs creates a set of directories from the manifest's claims.
func manifestDirs(m map[string]Artefact) *set.Set[string] {
	out := set.New[string]()
	for claim := range m {
		claim = path.Clean(filepath.ToSlash(claim))
		if path.IsAbs(claim) || isRel(claim) {
			continue
		}

		dir := path.Dir(claim)
		for dir != "." && dir != "/" {
			out.Add(dir)
			dir = path.Dir(dir)
		}
	}

	out.Add(".")

	return out
}

// makeArtefacts converts a list of artefacts into a map, and a collection of conflicts.
func makeArtefacts(as []Artefact) (artefacts map[string]Artefact, conflicts map[string][]Claim) {
	artefacts = make(map[string]Artefact)
	conflicts = make(map[string][]Claim)

	for _, a := range as {
		conflicts[a.Claim.Target] = append(conflicts[a.Claim.Target], a.Claim)
		artefacts[a.Claim.Target] = a
	}
	for d, cs := range conflicts {
		if len(cs) <= 1 {
			delete(conflicts, d)
		}
	}

	return artefacts, conflicts
}

func displayPath(root, rel string) string {
	if rel == "." {
		return root
	}
	return filepath.Clean(filepath.Join(root, rel))
}
