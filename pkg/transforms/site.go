package transforms

import (
	"slices"
	"sync"
	"time"
)

type Site struct {
	Title       string
	Description string
	URL         string

	Params map[string]any

	Meta  SiteMeta
	Pages *SitePages
}

type SitePages struct {
	pages []*Page
	once  sync.Once
	cache sitePagesCache
}

type sitePagesCache struct {
	All             []*Page
	Published       []*Page
	Drafts          []*Page
	Featured        []*Page
	Linked          []*Page
	Latest          []*Page
	RecentlyUpdated []*Page
	Undated         []*Page
	BySlug          map[string]*Page
	BySection       map[string][]*Page
	ByTag           map[string][]*Page
	ByYear          map[int][]*Page
	ByYearMonth     map[string][]*Page
}

func NewSitePages(pages []*Page) *SitePages {
	return &SitePages{pages: pages}
}

func (sp *SitePages) All() []*Page {
	sp.build()
	return sp.cache.All
}

func (sp *SitePages) Published() []*Page {
	sp.build()
	return sp.cache.Published
}

func (sp *SitePages) Drafts() []*Page {
	sp.build()
	return sp.cache.Drafts
}

func (sp *SitePages) Featured() []*Page {
	sp.build()
	return sp.cache.Featured
}

func (sp *SitePages) Linked() []*Page {
	sp.build()
	return sp.cache.Linked
}

func (sp *SitePages) Latest() []*Page {
	sp.build()
	return sp.cache.Latest
}

func (sp *SitePages) RecentlyUpdated() []*Page {
	sp.build()
	return sp.cache.RecentlyUpdated
}

func (sp *SitePages) Undated() []*Page {
	sp.build()
	return sp.cache.Undated
}

func (sp *SitePages) BySlug() map[string]*Page {
	sp.build()
	return sp.cache.BySlug
}

func (sp *SitePages) BySection() map[string][]*Page {
	sp.build()
	return sp.cache.BySection
}

func (sp *SitePages) ByTag() map[string][]*Page {
	sp.build()
	return sp.cache.ByTag
}

func (sp *SitePages) ByYear() map[int][]*Page {
	sp.build()
	return sp.cache.ByYear
}

func (sp *SitePages) ByYearMonth() map[string][]*Page {
	sp.build()
	return sp.cache.ByYearMonth
}

func (sp *SitePages) build() {
	sp.once.Do(func() {
		cache := sitePagesCache{
			All:             make([]*Page, 0),
			Published:       make([]*Page, 0),
			Drafts:          make([]*Page, 0),
			Featured:        make([]*Page, 0),
			Linked:          make([]*Page, 0),
			Latest:          make([]*Page, 0),
			RecentlyUpdated: make([]*Page, 0),
			Undated:         make([]*Page, 0),
			BySlug:          make(map[string]*Page),
			BySection:       make(map[string][]*Page),
			ByTag:           make(map[string][]*Page),
			ByYear:          make(map[int][]*Page),
			ByYearMonth:     make(map[string][]*Page),
		}

		if sp.pages == nil {
			sp.cache = cache
			return
		}

		for _, page := range sp.pages {
			if page == nil || page.HasError() {
				continue
			}

			cache.All = append(cache.All, page)

			if page.Featured {
				cache.Featured = append(cache.Featured, page)
			}

			if page.Draft {
				cache.Drafts = append(cache.Drafts, page)
			} else {
				cache.Published = append(cache.Published, page)
			}

			if len(page.Links) > 0 {
				cache.Linked = append(cache.Linked, page)
			}

			if page.Date.IsZero() {
				cache.Undated = append(cache.Undated, page)
			} else {
				year := page.Date.Year()
				cache.ByYear[year] = append(cache.ByYear[year], page)
				yearMonth := page.Date.Format("2006-01")
				cache.ByYearMonth[yearMonth] = append(cache.ByYearMonth[yearMonth], page)
			}

			if page.Slug != "" {
				cache.BySlug[page.Slug] = page
			}
			if page.Section != "" {
				cache.BySection[page.Section] = append(cache.BySection[page.Section], page)
			}
			for _, tag := range page.Tags {
				if tag != "" {
					cache.ByTag[tag] = append(cache.ByTag[tag], page)
				}
			}
		}

		cache.Latest = slices.Clone(cache.All)
		cache.RecentlyUpdated = slices.Clone(cache.All)
		slices.SortStableFunc(cache.Latest, func(a, b *Page) int { return b.PubDate.Compare(a.PubDate) })
		slices.SortStableFunc(cache.RecentlyUpdated, func(a, b *Page) int { return b.Updated.Compare(a.Updated) })

		sp.cache = cache
	})
}

type SiteMeta struct {
	ConfigPath string
	IsDev      bool
	Git        SiteGitMeta

	BuildTime time.Time
}
