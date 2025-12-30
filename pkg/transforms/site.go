package transforms

type Collections struct {
	All []*PageLite

	Drafts   []*PageLite
	Featured []*PageLite

	Latest          []*PageLite
	RecentlyUpdated []*PageLite
}

type Site struct {
	Title       string
	Description string
	URL         string

	Collections Collections
}
