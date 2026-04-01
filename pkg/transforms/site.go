package transforms

import "time"

// Site represents the global data for the site
type Site struct {
	Title       string
	Description string
	URL         string

	Params map[string]any

	Tree *PageTree

	Meta SiteMeta

	Collections Collections
	Groups      Groups
}

// Collections store collections of pages for Site
type Collections struct {
	All []*PageLite

	Published []*PageLite
	Drafts    []*PageLite
	Featured  []*PageLite

	Latest          []*PageLite
	RecentlyUpdated []*PageLite
	Undated         []*PageLite
}

// Groups store keyed page groupings for Site
type Groups struct {
	BySlug      map[string]*PageLite
	BySection   map[string][]*PageLite
	ByTag       map[string][]*PageLite
	ByYear      map[int][]*PageLite
	ByYearMonth map[string][]*PageLite
}

// SiteMeta stores metadata for the site
type SiteMeta struct {
	ConfigPath string
	IsDev      bool
	Git        SiteGitMeta

	BuildTime       time.Time
	BuildTimeString string
}
