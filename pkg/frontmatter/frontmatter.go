package frontmatter

import (
	"maps"
	"slices"
	"time"
)

type Frontmatter struct {
	Title       string   `toml:"title" yaml:"title" json:"title"`
	Description string   `toml:"description" yaml:"description" json:"description"`
	Section     string   `toml:"section" yaml:"section" json:"section"`
	Slug        string   `toml:"slug" yaml:"slug" json:"slug"`
	Tags        []string `toml:"tags" yaml:"tags" json:"tags"`

	Created time.Time `toml:"created" yaml:"created" json:"created"`
	Updated time.Time `toml:"updated" yaml:"updated" json:"updated"`

	RSS     RSSMeta     `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap SitemapMeta `toml:"sitemap" yaml:"sitemap" json:"sitemap"`
	Robots  RobotsMeta  `toml:"robots" yaml:"robots" json:"robots"`

	Params map[string]any `toml:"params" yaml:"params" json:"params"`

	Template string `toml:"template" yaml:"template" json:"template"`

	Featured bool `toml:"featured" yaml:"featured" json:"featured"`
	Draft    bool `toml:"draft" yaml:"draft" json:"draft"`
	Weight   int  `toml:"weight" yaml:"weight" json:"weight"`
}

type RSSMeta struct {
	Include     bool   `toml:"include" yaml:"include" json:"include"`
	Title       string `toml:"title" yaml:"title" json:"title"`
	Description string `toml:"description" yaml:"description" json:"description"`
	GUID        string `toml:"guid" yaml:"guid" json:"guid"`
}

type SitemapMeta struct {
	Include    bool    `toml:"include" yaml:"include" json:"include"`
	ChangeFreq string  `toml:"changefreq" yaml:"changefreq" json:"changefreq"`
	Priority   float64 `toml:"priority" yaml:"priority" json:"priority"`
}

type RobotsMeta struct {
	Disallow bool `toml:"disallow" yaml:"disallow" json:"disallow"`
}

func (fm *Frontmatter) Clone() *Frontmatter {
	clone := *fm
	clone.Tags = slices.Clone(fm.Tags)
	clone.Params = maps.Clone(fm.Params)
	return &clone
}
