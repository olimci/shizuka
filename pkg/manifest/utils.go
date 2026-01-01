package manifest

import (
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
		dir, base := filepath.Split(p)
		if base == ".." {
			return true
		}
		if dir == "" || dir == string(filepath.Separator) {
			return false
		}
		p = strings.TrimSuffix(dir, string(filepath.Separator))
	}
}

// manifestDirs creates a set of directories from the manifest's claims
func manifestDirs(m map[string]ArtefactBuilder) *set.Set[string] {
	out := set.New[string]()
	for claim := range m {
		claim = filepath.Clean(claim)
		if filepath.IsAbs(claim) || isRel(claim) {
			continue
		}

		dir := filepath.Dir(claim)
		for dir != "." && dir != string(filepath.Separator) {
			out.Add(dir)
			dir = filepath.Dir(dir)
		}
	}

	out.Add(".")

	return out
}

// makeArtefacts converts a list of artefacts into a map, and a collection of conflicts.
func makeArtefacts(as []Artefact) (artefacts map[string]ArtefactBuilder, conflicts map[string][]Claim) {
	artefacts = make(map[string]ArtefactBuilder)
	conflicts = make(map[string][]Claim)

	for _, a := range as {
		conflicts[a.Claim.Target] = append(conflicts[a.Claim.Target], a.Claim)
		artefacts[a.Claim.Target] = a.Builder
	}
	for d, cs := range conflicts {
		if len(cs) <= 1 {
			delete(conflicts, d)
		}
	}

	return artefacts, conflicts
}
