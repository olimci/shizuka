package build

import (
	"errors"
	"html/template"
)

var ErrPageBuild = errors.New("page build error")

func lookupErrPage(o *Options, err error) *template.Template {
	if o == nil || !o.Dev || err == nil {
		return nil
	}

	for match, tmpl := range o.ErrPages {
		if errors.Is(err, match) {
			return tmpl
		}
	}

	return o.DefaultErrPage
}
