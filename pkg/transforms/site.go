package transforms

import "time"

// Site represents the global data for the site
type Site struct {
	Title       string
	Description string
	URL         string

	Meta SiteMeta

	Collections Collections
}

// TODO: page tree collection

// Collections store collections of pages for Site
type Collections struct {
	All []*PageLite

	Drafts   []*PageLite
	Featured []*PageLite

	Latest          []*PageLite
	RecentlyUpdated []*PageLite
}

// SiteMeta stores metadata for the site
type SiteMeta struct {
	ConfigPath string
	IsDev      bool

	BuildTime       time.Time
	BuildTimeString string
}
