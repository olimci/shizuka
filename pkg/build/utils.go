package build

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/tdewolff/minify/v2"
	mincss "github.com/tdewolff/minify/v2/css"
	minhtml "github.com/tdewolff/minify/v2/html"
	minjs "github.com/tdewolff/minify/v2/js"
)

func newMinifier(enabled bool) *minify.M {
	if !enabled {
		return nil
	}

	m := minify.New()

	m.AddFunc("text/html", minhtml.Minify)
	m.AddFunc("text/css", mincss.Minify)
	m.AddFunc("application/javascript", minjs.Minify)

	return m
}

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

func minifyArtefact(m *minify.M, target string, artefact manifest.Artefact) manifest.Artefact {
	if m == nil {
		return artefact
	}

	mimes := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
	}

	if mime, ok := mimes[filepath.Ext(filepath.Base(target))]; ok {
		return manifest.Artefact{
			Claim: artefact.Claim.AddTag("minified"),
			Builder: func(w io.Writer) error {
				x := m.Writer(mime, w)
				defer x.Close()
				return artefact.Builder(x)
			},
		}
	} else {
		return artefact
	}
}

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
