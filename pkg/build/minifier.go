package build

import (
	"io"
	"path/filepath"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/tdewolff/minify/v2"
	mincss "github.com/tdewolff/minify/v2/css"
	minhtml "github.com/tdewolff/minify/v2/html"
	minjs "github.com/tdewolff/minify/v2/js"
)

type Minifier struct {
	m     *minify.M
	mimes map[string]string
}

type MinifierOption func(*Minifier)

func WithMinifierMIME(ext string, mime string) MinifierOption {
	return func(m *Minifier) {
		if m.mimes == nil {
			m.mimes = make(map[string]string)
		}
		m.mimes[ext] = mime
	}
}

func NewMinifier(enabled bool, opts ...MinifierOption) *Minifier {
	if !enabled {
		return nil
	}

	m := minify.New()
	m.AddFunc("text/html", minhtml.Minify)
	m.AddFunc("text/css", mincss.Minify)
	m.AddFunc("application/javascript", minjs.Minify)

	out := &Minifier{
		m: m,
		mimes: map[string]string{
			".html": "text/html",
			".css":  "text/css",
			".js":   "application/javascript",
		},
	}

	for _, opt := range opts {
		opt(out)
	}

	return out
}

func (m *Minifier) MinifyArtefact(target string, artefact manifest.Artefact) manifest.Artefact {
	if m == nil || m.m == nil {
		return artefact
	}

	if mime, ok := m.mimes[filepath.Ext(filepath.Base(target))]; ok {
		return manifest.Artefact{
			Claim: artefact.Claim.AddTag("minified"),
			Builder: func(w io.Writer) error {
				x := m.m.Writer(mime, w)
				defer x.Close()
				return artefact.Builder(x)
			},
		}
	}

	return artefact
}
