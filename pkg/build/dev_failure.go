package build

import (
	"io"

	"github.com/olimci/shizuka/pkg/manifest"
)

type DevFailurePageData struct {
	Summary     string
	FailLevel   DiagnosticLevel
	MaxLevel    DiagnosticLevel
	Diagnostics []Diagnostic
}

func defaultDevFailureTarget(o *Options) string {
	if o != nil && o.DevFailureTarget != "" {
		return o.DevFailureTarget
	}
	return "index.html"
}

func devFailureArtefact(o *Options, data DevFailurePageData) *manifest.Artefact {
	if o == nil || !o.Dev || o.DevFailureTemplate == nil {
		return nil
	}

	target := defaultDevFailureTarget(o)
	a := manifest.Artefact{
		Claim: manifest.Claim{
			Owner:  "build:failure",
			Source: "build",
			Target: target,
			Tags:   []string{"dev", "error"},
		},
		Builder: func(w io.Writer) error {
			return o.DevFailureTemplate.Execute(w, data)
		},
	}

	return &a
}
