package build

import (
	"html/template"

	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/structql"
)

const (
	PagesK        = registry.K[[]*transforms.Page]("pages")
	ContentFilesK = registry.K[[]string]("contentfiles")
	SiteK         = registry.K[*transforms.Site]("site")
	DBK           = registry.K[*structql.DB]("db")
	TemplatesK    = registry.K[*template.Template]("templates")
	BuildCtxK     = registry.K[*BuildCtx]("buildctx")
	SiteGitK      = registry.K[*transforms.SiteGitMeta]("sitegit")

	StaticCacheK     = registry.K[*staticStepCache]("cache:static")
	PageIndexCacheK  = registry.K[*pageIndexStepCache]("cache:pages:index")
	PageAssetsCacheK = registry.K[*pageAssetsStepCache]("cache:pages:assets")
	GitCacheK        = registry.K[*gitStepCache]("cache:git")
)
