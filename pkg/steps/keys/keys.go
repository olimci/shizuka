package keys

import (
	"html/template"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
)

const (
	Config    = manifest.K[*config.Config]("config")
	Options   = manifest.K[*config.Options]("options")
	Pages     = manifest.K[*transforms.PageTree]("pages")
	Site      = manifest.K[*transforms.Site]("site")
	Templates = manifest.K[*template.Template]("templates")
	BuildMeta = manifest.K[*transforms.BuildMeta]("buildmeta")
)
