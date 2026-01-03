package build

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
)

// static creates a new static artefact
func makeStatic(owner, source, target string) manifest.Artefact {
	return manifest.Artefact{
		Claim: manifest.Claim{
			Owner:  owner,
			Source: source,
			Target: target,
		},
		Builder: func(w io.Writer) error {
			file, err := os.Open(source)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(w, file)
			return err
		},
	}
}

// makeTarget creates a new target path based on the root and relative path
func makeTarget(root, rel string) (src, dst string, err error) {
	dir, base := filepath.Split(rel)

	name := strings.TrimSuffix(base, filepath.Ext(base))

	src = filepath.Join(root, rel)

	if name == "index" {
		return src, filepath.Join(dir, "index.html"), nil
	} else {
		return src, filepath.Join(dir, name, "index.html"), nil
	}
}
