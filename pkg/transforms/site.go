package transforms

import "time"

type Site struct {
	Title       string
	Description string
	URL         string

	Params map[string]any

	Meta SiteMeta
}

type SiteMeta struct {
	ConfigPath string
	IsDev      bool
	Git        SiteGitMeta

	BuildTime time.Time
}
