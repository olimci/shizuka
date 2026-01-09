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

func NewMinifier(enabled bool) manifest.PostProcessor {
	if !enabled {
		return nil
	}

	mimes := map[string]string{
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
	}

	m := minify.New()
	m.AddFunc("text/html", minhtml.Minify)
	m.AddFunc("text/css", mincss.Minify)
	m.AddFunc("application/javascript", minjs.Minify)

	return func(claim manifest.Claim, next manifest.ArtefactBuilder) manifest.ArtefactBuilder {
		mime, ex := mimes[filepath.Ext(claim.Target)]
		if !ex {
			return next
		}

		return func(w io.Writer) error {
			x := m.Writer(mime, w)
			if err := next(x); err != nil {
				return err
			}
			return x.Close()
		}
	}
}
