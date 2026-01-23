package manifest

import (
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/iofs"
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

// manifestDirs creates a set of directories from the manifest's claims
func manifestDirs(m map[string]ArtefactBuilder) *set.Set[string] {
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

func walkDestination(ctx context.Context, out iofs.Writable) (*set.Set[string], *set.Set[string], error) {
	if ctx == nil {
		ctx = context.Background()
	}
	fsys, err := out.FS(ctx)
	if err != nil {
		return nil, nil, err
	}

	files := set.New[string]()
	dirs := set.New[string]()

	err = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		p = path.Clean(p)
		if p == "." {
			dirs.Add(".")
			return nil
		}

		if d.IsDir() {
			dirs.Add(p)
			return nil
		}

		files.Add(p)
		return nil
	})

	return files, dirs, err
}

func displayPath(out iofs.Writable, rel string) string {
	type displayer interface {
		DisplayPath(string) string
	}
	if d, ok := out.(displayer); ok {
		return d.DisplayPath(rel)
	}

	root := strings.TrimSpace(out.Root())
	if root == "" || root == "." {
		return rel
	}
	return path.Join(root, rel)
}
