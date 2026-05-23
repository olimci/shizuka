package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/olimci/roundtrip/json"
	"github.com/olimci/shizuka/pkg/frontmatter"
	"github.com/olimci/shizuka/pkg/utils/urlutil"
	"github.com/olimci/shizuka/pkg/version"
	gm "github.com/yuin/goldmark"
	gmext "github.com/yuin/goldmark/extension"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

const SchemaURL = "https://raw.githubusercontent.com/olimci/shizuka/refs/heads/main/_assets/config.schema.json"

// Load loads a Config from a file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewJSONCDecoder(file)
	decoder.DisallowUnknownFields()
	if _, err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	cfg.Root = filepath.Dir(filepath.Clean(path))
	if cfg.Root == "" {
		cfg.Root = "."
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	return cfg, nil
}

// Config represents the site configuration.
type Config struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`

	Site      ConfigSite      `json:"site"`
	Paths     ConfigPaths     `json:"paths"`
	Build     ConfigBuild     `json:"build"`
	Content   ConfigContent   `json:"content"`
	Artefacts ConfigArtefacts `json:"artefacts"`

	Root string `toml:"-" yaml:"-" json:"-"`
}

type ConfigSite struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`

	Params map[string]any `json:"params"`
}

type ConfigPaths struct {
	Output    string `json:"output"`
	Content   string `json:"content"`
	Data      string `json:"data"`
	Static    string `json:"static"`
	Templates string `json:"templates"`
}

type ConfigBuild struct {
	Minifier *ConfigMinifier `json:"minifier"`
}

type ConfigMinifier struct {
	Whitelist []string `json:"whitelist"`
	Blacklist []string `json:"blacklist"`
}

type ConfigContent struct {
	Defaults ConfigContentDefaults `json:"defaults"`
	Markdown ConfigContentMarkdown `json:"markdown"`
	Git      *ConfigContentGit     `json:"git"`
}

type ConfigContentDefaults struct {
	Section  string                          `json:"section"`
	Global   frontmatter.Defaults            `json:"global"`
	Sections map[string]frontmatter.Defaults `json:"sections"`
}

type ConfigContentMarkdown struct {
	Goldmark ConfigGoldmark `json:"goldmark"`
}

type ConfigContentGit struct {
	Backfill bool `json:"backfill"`
}

type ConfigArtefacts struct {
	Headers   *ConfigHeaders   `json:"headers"`
	Redirects *ConfigRedirects `json:"redirects"`
	RSS       *ConfigRSS       `json:"rss"`
	Sitemap   *ConfigSitemap   `json:"sitemap"`
	Robots    *ConfigRobots    `json:"robots"`
	NotFound  *ConfigNotFound  `json:"not_found"`
	Meta      *ConfigMeta      `json:"meta"`
}

type ConfigHeaders struct {
	Path   string                       `json:"path"`
	Values map[string]map[string]string `json:"values"`
}

type ConfigRedirects struct {
	Path    string     `json:"path"`
	Entries []Redirect `json:"entries"`
}

type ConfigRSS struct {
	Path          string   `json:"path"`
	Sections      []string `json:"sections"`
	Limit         int      `json:"limit"`
	IncludeDrafts bool     `json:"include_drafts"`
}

type ConfigSitemap struct {
	Path          string `json:"path"`
	IncludeDrafts bool   `json:"include_drafts"`
}

type ConfigRobots struct {
	Path           string        `json:"path"`
	IncludeSitemap bool          `json:"include_sitemap"`
	IncludeDrafts  bool          `json:"include_drafts"`
	Sitemaps       []string      `json:"sitemaps"`
	Groups         []RobotsGroup `json:"groups"`
}

type RobotsGroup struct {
	UserAgents []string `json:"user_agents"`
	Allow      []string `json:"allow"`
	Disallow   []string `json:"disallow"`
}

type ConfigNotFound struct {
	Path     string `json:"path"`
	Template string `json:"template"`
}

type ConfigMeta struct {
	Path string `json:"path"`
	JSON bool   `json:"json"`
}

type Redirect struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Status int    `json:"status"`
}

type ConfigGoldmark struct {
	Extensions []string               `json:"extensions"`
	Parser     ConfigGoldmarkParser   `json:"parser"`
	Renderer   ConfigGoldmarkRenderer `json:"renderer"`
}

type ConfigGoldmarkParser struct {
	AutoHeadingID bool `json:"auto_heading_id"`
	Attribute     bool `json:"attribute"`
}

type ConfigGoldmarkRenderer struct {
	Hardbreaks bool `json:"hardbreaks"`
	XHTML      bool `json:"XHTML"`
}

// DefaultConfig constructs a new Config with default values.
func DefaultConfig() *Config {
	defaultGoldmark := ConfigGoldmark{
		Extensions: []string{
			"gfm",
			"table",
			"strikethrough",
			"tasklist",
			"deflist",
			"footnotes",
			"typographer",
		},
		Parser: ConfigGoldmarkParser{
			AutoHeadingID: false,
			Attribute:     false,
		},
		Renderer: ConfigGoldmarkRenderer{
			Hardbreaks: false,
			XHTML:      false,
		},
	}

	return &Config{
		Schema:  SchemaURL,
		Version: version.String(),
		Site: ConfigSite{
			Title:       "Shizuka",
			Description: "Shizuka site",
			URL:         "https://example.com",
			Params:      map[string]any{},
		},
		Paths: ConfigPaths{
			Output:    "dist",
			Content:   "content",
			Data:      "data",
			Static:    "static",
			Templates: "templates/*.tmpl",
		},
		Build: ConfigBuild{
			Minifier: &ConfigMinifier{},
		},
		Content: ConfigContent{
			Defaults: ConfigContentDefaults{
				Section: "pages",
				Global: frontmatter.Defaults{
					Template: "page",
				},
			},
			Markdown: ConfigContentMarkdown{
				Goldmark: defaultGoldmark,
			},
		},
	}
}

// Validate validates the Config.
func (c *Config) Validate() error {
	if c.Schema == "" {
		c.Schema = SchemaURL
	}
	if c.Version == "" {
		c.Version = version.String()
	}
	if err := version.CheckCompatible(c.Version); err != nil {
		return err
	}

	siteURL, err := urlutil.ValidURL(c.Site.URL)
	if err != nil {
		return fmt.Errorf("site.url: %w", err)
	}
	c.Site.URL = siteURL

	if c.Site.Params == nil {
		c.Site.Params = map[string]any{}
	}
	if c.Content.Defaults.Sections == nil {
		c.Content.Defaults.Sections = map[string]frontmatter.Defaults{}
	}

	if c.Build.Minifier != nil {
		patterns, err := cleanPatterns("build.minifier.whitelist", c.Build.Minifier.Whitelist)
		if err != nil {
			return err
		}
		c.Build.Minifier.Whitelist = patterns

		patterns, err = cleanPatterns("build.minifier.blacklist", c.Build.Minifier.Blacklist)
		if err != nil {
			return err
		}
		c.Build.Minifier.Blacklist = patterns
	}

	if c.Artefacts.Headers != nil {
		if c.Artefacts.Headers.Values == nil {
			c.Artefacts.Headers.Values = map[string]map[string]string{}
		}
		if c.Artefacts.Headers.Path == "" {
			c.Artefacts.Headers.Path = "_headers"
		}
		path, err := c.resolvePath("artefacts.headers.path", c.Artefacts.Headers.Path)
		if err != nil {
			return err
		}
		c.Artefacts.Headers.Path = path
	}

	if c.Artefacts.Redirects != nil {
		if c.Artefacts.Redirects.Path == "" {
			c.Artefacts.Redirects.Path = "_redirects"
		}
		path, err := c.resolvePath("artefacts.redirects.path", c.Artefacts.Redirects.Path)
		if err != nil {
			return err
		}
		c.Artefacts.Redirects.Path = path
	}

	if c.Artefacts.RSS != nil && c.Artefacts.RSS.Path == "" {
		c.Artefacts.RSS.Path = "rss.xml"
	}
	if c.Artefacts.RSS != nil {
		path, err := c.resolvePath("artefacts.rss.path", c.Artefacts.RSS.Path)
		if err != nil {
			return err
		}
		c.Artefacts.RSS.Path = path
	}
	if c.Artefacts.Sitemap != nil && c.Artefacts.Sitemap.Path == "" {
		c.Artefacts.Sitemap.Path = "sitemap.xml"
	}
	if c.Artefacts.Sitemap != nil {
		path, err := c.resolvePath("artefacts.sitemap.path", c.Artefacts.Sitemap.Path)
		if err != nil {
			return err
		}
		c.Artefacts.Sitemap.Path = path
	}
	if c.Artefacts.Robots != nil {
		if c.Artefacts.Robots.Path == "" {
			c.Artefacts.Robots.Path = "robots.txt"
		}
		path, err := c.resolvePath("artefacts.robots.path", c.Artefacts.Robots.Path)
		if err != nil {
			return err
		}
		c.Artefacts.Robots.Path = path
		for i, group := range c.Artefacts.Robots.Groups {
			if len(group.UserAgents) == 0 {
				group.UserAgents = []string{"*"}
			}
			c.Artefacts.Robots.Groups[i] = group
		}
	}
	if c.Artefacts.NotFound != nil && c.Artefacts.NotFound.Path == "" {
		c.Artefacts.NotFound.Path = "404.html"
	}
	if c.Artefacts.NotFound != nil {
		path, err := c.resolvePath("artefacts.not_found.path", c.Artefacts.NotFound.Path)
		if err != nil {
			return err
		}
		c.Artefacts.NotFound.Path = path
	}
	if c.Artefacts.Meta != nil && c.Artefacts.Meta.Path == "" {
		c.Artefacts.Meta.Path = "_shizuka/index.html"
	}
	if c.Artefacts.Meta != nil {
		path, err := c.resolvePath("artefacts.meta.path", c.Artefacts.Meta.Path)
		if err != nil {
			return err
		}
		c.Artefacts.Meta.Path = path
	}

	outputPath, err := c.resolvePath("paths.output", c.Paths.Output)
	if err != nil {
		return err
	}
	c.Paths.Output = outputPath

	staticPath, err := c.resolvePath("paths.static", c.Paths.Static)
	if err != nil {
		return err
	}
	c.Paths.Static = staticPath

	contentPath, err := c.resolvePath("paths.content", c.Paths.Content)
	if err != nil {
		return err
	}
	c.Paths.Content = contentPath

	dataPath, err := c.resolvePath("paths.data", c.Paths.Data)
	if err != nil {
		return err
	}
	c.Paths.Data = dataPath

	templateGlob, err := c.resolveGlob("paths.templates", c.Paths.Templates)
	if err != nil {
		return err
	}
	c.Paths.Templates = templateGlob

	return nil
}

func (c *Config) WatchedPaths() (paths []string, globs []string, err error) {
	return []string{
		filepath.Join(c.root(), filepath.FromSlash(c.StaticSourcePath())),
		filepath.Join(c.root(), filepath.FromSlash(c.ContentSourcePath())),
		filepath.Join(c.root(), filepath.FromSlash(c.Paths.Data)),
	}, []string{filepath.Join(c.root(), filepath.FromSlash(c.TemplateGlob()))}, nil
}

func (c *Config) OutputPath() string {
	return filepath.Join(c.root(), filepath.FromSlash(c.Paths.Output))
}

func (c *Config) StaticSourcePath() string {
	return c.Paths.Static
}

func (c *Config) ContentSourcePath() string {
	return c.Paths.Content
}

func (c *Config) TemplateGlob() string {
	return c.Paths.Templates
}

func (cfg ConfigGoldmark) Build() gm.Markdown {
	var (
		exts       []gm.Extender
		parserOpts []gmparse.Option
		htmlOpts   []gmrenderer.Option
	)

	for _, name := range cfg.Extensions {
		switch name {
		case "gfm":
			exts = append(exts, gmext.GFM)
		case "table", "tables":
			exts = append(exts, gmext.Table)
		case "strikethrough":
			exts = append(exts, gmext.Strikethrough)
		case "tasklist", "task-list":
			exts = append(exts, gmext.TaskList)
		case "deflist", "definition-list":
			exts = append(exts, gmext.DefinitionList)
		case "footnote", "footnotes":
			exts = append(exts, gmext.Footnote)
		case "linkify":
			exts = append(exts, gmext.Linkify)
		case "typographer", "smartypants":
			exts = append(exts, gmext.Typographer)
		default:
		}
	}

	if cfg.Parser.AutoHeadingID {
		parserOpts = append(parserOpts, gmparse.WithAutoHeadingID())
	}
	if cfg.Parser.Attribute {
		parserOpts = append(parserOpts, gmparse.WithAttribute())
	}

	htmlOpts = append(htmlOpts, gmhtml.WithUnsafe())

	if cfg.Renderer.Hardbreaks {
		htmlOpts = append(htmlOpts, gmhtml.WithHardWraps())
	}
	if cfg.Renderer.XHTML {
		htmlOpts = append(htmlOpts, gmhtml.WithXHTML())
	}

	opts := make([]gm.Option, 0, 3)
	if len(exts) > 0 {
		opts = append(opts, gm.WithExtensions(exts...))
	}
	if len(parserOpts) > 0 {
		opts = append(opts, gm.WithParserOptions(parserOpts...))
	}
	if len(htmlOpts) > 0 {
		opts = append(opts, gm.WithRendererOptions(htmlOpts...))
	}

	return gm.New(opts...)
}
