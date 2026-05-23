package transforms

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
)

type RobotsTemplateData struct {
	Groups   []RobotsGroup
	Sitemaps []string
}

type RobotsGroup struct {
	UserAgents []string
	Allow      []string
	Disallow   []string
}

func BuildRobots(pages []*Page, site *Site, cfg *config.ConfigRobots, sitemapCfg *config.ConfigSitemap) RobotsTemplateData {
	groups := make([]RobotsGroup, 0, len(cfg.Groups)+1)
	for _, group := range cfg.Groups {
		groups = append(groups, RobotsGroup{
			UserAgents: slices.Clone(group.UserAgents),
			Allow:      slices.Clone(group.Allow),
			Disallow:   slices.Clone(group.Disallow),
		})
	}

	pageDisallows := make([]string, 0)
	for _, page := range pages {
		if page == nil || page.Error != nil {
			continue
		}
		if !cfg.IncludeDrafts && page.Draft {
			continue
		}
		if !page.Robots.Disallow {
			continue
		}
		pageDisallows = append(pageDisallows, page.Path)
	}
	if len(pageDisallows) > 0 {
		groups = append(groups, RobotsGroup{
			UserAgents: []string{"*"},
			Disallow:   pageDisallows,
		})
	}

	sitemaps := slices.Clone(cfg.Sitemaps)
	if cfg.IncludeSitemap && sitemapCfg != nil {
		sitemaps = append(sitemaps, siteAbsURL(site, sitemapCfg.Path))
	}

	return RobotsTemplateData{
		Groups:   groups,
		Sitemaps: sitemaps,
	}
}

func RenderRobots(data RobotsTemplateData) string {
	var out strings.Builder

	for i, group := range data.Groups {
		if i > 0 {
			out.WriteByte('\n')
		}
		for _, agent := range group.UserAgents {
			fmt.Fprintf(&out, "User-agent: %s\n", agent)
		}
		for _, item := range group.Allow {
			fmt.Fprintf(&out, "Allow: %s\n", item)
		}
		for _, item := range group.Disallow {
			fmt.Fprintf(&out, "Disallow: %s\n", item)
		}
	}

	if len(data.Sitemaps) > 0 {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		for _, sitemap := range data.Sitemaps {
			fmt.Fprintf(&out, "Sitemap: %s\n", sitemap)
		}
	}

	return out.String()
}

func siteAbsURL(site *Site, rel string) string {
	if site == nil {
		return rel
	}
	base, err := url.Parse(site.URL)
	if err != nil {
		return rel
	}
	ref, err := url.Parse(rel)
	if err != nil {
		return rel
	}
	return base.ResolveReference(ref).String()
}
