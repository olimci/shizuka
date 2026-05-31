package frontmatter

import (
	"slices"
	"time"
)

type Defaults struct {
	Title       string    `toml:"title" yaml:"title" json:"title"`
	Description string    `toml:"description" yaml:"description" json:"description"`
	Slug        string    `toml:"slug" yaml:"slug" json:"slug"`
	Tags        []string  `toml:"tags" yaml:"tags" json:"tags"`
	Created     time.Time `toml:"created" yaml:"created" json:"created"`
	Updated     time.Time `toml:"updated" yaml:"updated" json:"updated"`

	RSS     RSSMeta     `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap SitemapMeta `toml:"sitemap" yaml:"sitemap" json:"sitemap"`
	Robots  RobotsMeta  `toml:"robots" yaml:"robots" json:"robots"`

	Template string `toml:"template" yaml:"template" json:"template"`

	Featured bool `toml:"featured" yaml:"featured" json:"featured"`
	Draft    bool `toml:"draft" yaml:"draft" json:"draft"`
	Weight   int  `toml:"weight" yaml:"weight" json:"weight"`
}

func (d Defaults) Frontmatter() Frontmatter {
	return Frontmatter{
		Title:       d.Title,
		Description: d.Description,
		Slug:        d.Slug,
		Tags:        slices.Clone(d.Tags),
		Created:     d.Created,
		Updated:     d.Updated,
		RSS:         d.RSS,
		Sitemap:     d.Sitemap,
		Robots:      d.Robots,
		Template:    d.Template,
		Featured:    d.Featured,
		Draft:       d.Draft,
		Weight:      d.Weight,
		Params:      map[string]any{},
	}
}
