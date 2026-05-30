package build

import (
	"html/template"

	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/structql"
)

const (
	PagesK     = registry.K[[]*transforms.Page]("pages")
	SiteK      = registry.K[*transforms.Site]("site")
	DBK        = registry.K[*structql.DB]("db")
	TemplatesK = registry.K[*template.Template]("templates")
	BuildCtxK  = registry.K[*BuildCtx]("buildctx")
	SiteGitK   = registry.K[*transforms.SiteGitMeta]("sitegit")

	GitCacheK     = registry.K[*gitStepCache]("cache:git")
	ChangedPathsK = registry.K[[]string]("cache:changed_paths")
)
