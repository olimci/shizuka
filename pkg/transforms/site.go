package transforms

// Site represents the global data for the site
type Site struct {
	Title       string
	Description string
	URL         string

	Meta SiteMeta

	Collections Collections
}

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
	BuildTime string
	Dev       bool
}
