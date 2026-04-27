package config

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/shizuka/pkg/version"

	gm "github.com/yuin/goldmark"
	gmext "github.com/yuin/goldmark/extension"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// Config represents the site configuration.
type Config struct {
	Version   string           `toml:"version" yaml:"version" json:"version"`
	Site      ConfigSite       `toml:"site" yaml:"site" json:"site"`
	Paths     ConfigPaths      `toml:"paths" yaml:"paths" json:"paths"`
	Build     ConfigBuild      `toml:"build" yaml:"build" json:"build"`
	Content   ConfigContent    `toml:"content" yaml:"content" json:"content"`
	Headers   *ConfigHeaders   `toml:"headers" yaml:"headers" json:"headers"`
	Redirects *ConfigRedirects `toml:"redirects" yaml:"redirects" json:"redirects"`
	RSS       *ConfigRSS       `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap   *ConfigSitemap   `toml:"sitemap" yaml:"sitemap" json:"sitemap"`

	Root string `toml:"-" yaml:"-" json:"-"`
}

type ConfigSite struct {
	Title       string `toml:"title" yaml:"title" json:"title"`
	Description string `toml:"description" yaml:"description" json:"description"`
	URL         string `toml:"url" yaml:"url" json:"url"`

	Params map[string]any `toml:"params" yaml:"params" json:"params"`
}

type ConfigPaths struct {
	Output    string `toml:"output" yaml:"output" json:"output"`
	Content   string `toml:"content" yaml:"content" json:"content"`
	Static    string `toml:"static" yaml:"static" json:"static"`
	Templates string `toml:"templates" yaml:"templates" json:"templates"`
}

type ConfigBuild struct {
	Minify bool `toml:"minify" yaml:"minify" json:"minify"`
}

type ConfigContent struct {
	Defaults ConfigContentDefaults `toml:"defaults" yaml:"defaults" json:"defaults"`
	Markdown ConfigContentMarkdown `toml:"markdown" yaml:"markdown" json:"markdown"`
	Bundles  ConfigContentBundles  `toml:"bundles" yaml:"bundles" json:"bundles"`
	Raw      ConfigContentRaw      `toml:"raw" yaml:"raw" json:"raw"`
	Git      *ConfigContentGit     `toml:"git" yaml:"git" json:"git"`
}

type ConfigContentDefaults struct {
	Template string         `toml:"template" yaml:"template" json:"template"`
	Section  string         `toml:"section" yaml:"section" json:"section"`
	Params   map[string]any `toml:"params" yaml:"params" json:"params"`
}

type ConfigContentMarkdown struct {
	Wikilinks bool           `toml:"wikilinks" yaml:"wikilinks" json:"wikilinks"`
	Goldmark  ConfigGoldmark `toml:"goldmark" yaml:"goldmark" json:"goldmark"`
}

type ConfigContentBundles struct {
	Enabled bool   `toml:"enabled" yaml:"enabled" json:"enabled"`
	Output  string `toml:"output" yaml:"output" json:"output"`
	Mode    string `toml:"mode" yaml:"mode" json:"mode"`
}

type ConfigContentRaw struct {
	Markdown bool `toml:"markdown" yaml:"markdown" json:"markdown"`
}

type ConfigContentGit struct {
	Backfill bool `toml:"backfill" yaml:"backfill" json:"backfill"`
}

type ConfigHeaders struct {
	Output string                       `toml:"output" yaml:"output" json:"output"`
	Values map[string]map[string]string `toml:"values" yaml:"values" json:"values"`
}

type ConfigRedirects struct {
	Output  string     `toml:"output" yaml:"output" json:"output"`
	Shorten string     `toml:"shorten" yaml:"shorten" json:"shorten"`
	Entries []Redirect `toml:"entries" yaml:"entries" json:"entries"`
}

type ConfigRSS struct {
	Output        string   `toml:"output" yaml:"output" json:"output"`
	Sections      []string `toml:"sections" yaml:"sections" json:"sections"`
	Limit         int      `toml:"limit" yaml:"limit" json:"limit"`
	IncludeDrafts bool     `toml:"include_drafts" yaml:"include_drafts" json:"include_drafts"`
}

type ConfigSitemap struct {
	Output        string `toml:"output" yaml:"output" json:"output"`
	IncludeDrafts bool   `toml:"include_drafts" yaml:"include_drafts" json:"include_drafts"`
}

type Redirect struct {
	From   string `toml:"from" yaml:"from" json:"from"`
	To     string `toml:"to" yaml:"to" json:"to"`
	Status int    `toml:"status" yaml:"status" json:"status"`
}

type ConfigGoldmark struct {
	Extensions []string               `toml:"extensions" yaml:"extensions" json:"extensions"`
	Parser     ConfigGoldmarkParser   `toml:"parser" yaml:"parser" json:"parser"`
	Renderer   ConfigGoldmarkRenderer `toml:"renderer" yaml:"renderer" json:"renderer"`
}

type ConfigGoldmarkParser struct {
	AutoHeadingID bool `toml:"auto_heading_id" yaml:"auto_heading_id" json:"auto_heading_id"`
	Attribute     bool `toml:"attribute" yaml:"attribute" json:"attribute"`
}

type ConfigGoldmarkRenderer struct {
	Hardbreaks bool `toml:"hardbreaks" yaml:"hardbreaks" json:"hardbreaks"`
	XHTML      bool `toml:"XHTML" yaml:"XHTML" json:"XHTML"`
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
			Static:    "static",
			Templates: "templates/*.tmpl",
		},
		Build: ConfigBuild{
			Minify: true,
		},
		Content: ConfigContent{
			Defaults: ConfigContentDefaults{
				Template: "page",
				Params:   map[string]any{},
			},
			Markdown: ConfigContentMarkdown{
				Goldmark: defaultGoldmark,
			},
			Bundles: ConfigContentBundles{
				Enabled: false,
				Output:  "_assets",
				Mode:    "fingerprinted",
			},
		},
	}
}

// Load loads a Config from a file.
func Load(path string) (*Config, error) {
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	cfg := DefaultConfig()

	if err := decodeFile(resolvedPath, cfg); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	cfg.Root = filepath.Dir(filepath.Clean(resolvedPath))
	if cfg.Root == "" {
		cfg.Root = "."
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}
	return cfg, nil
}

// Validate validates the Config.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Version) == "" {
		c.Version = version.String()
	}

	if c.Site.URL == "" {
		return errors.New("site.url is required")
	}
	if !(strings.HasPrefix(c.Site.URL, "http://") || strings.HasPrefix(c.Site.URL, "https://")) {
		return fmt.Errorf("site.url must start with http:// or https:// (got %q)", c.Site.URL)
	}
	if _, err := url.Parse(c.Site.URL); err != nil {
		return fmt.Errorf("site.url is not a valid URL (got %q): %w", c.Site.URL, err)
	}

	if c.Site.Params == nil {
		c.Site.Params = map[string]any{}
	}
	if c.Content.Defaults.Params == nil {
		c.Content.Defaults.Params = map[string]any{}
	}

	if c.Headers != nil {
		if c.Headers.Values == nil {
			c.Headers.Values = map[string]map[string]string{}
		}
		if c.Headers.Output == "" {
			c.Headers.Output = "_headers"
		}
	}

	if c.Redirects != nil {
		shorten := c.Redirects.Shorten
		if shorten == "" {
			shorten = "/s"
		}
		if !strings.HasPrefix(shorten, "/") {
			shorten = "/" + shorten
		}
		shorten = strings.TrimSuffix(shorten, "/")
		c.Redirects.Shorten = shorten

		if c.Redirects.Output == "" {
			c.Redirects.Output = "_redirects"
		}
	}

	if c.RSS != nil && c.RSS.Output == "" {
		c.RSS.Output = "rss.xml"
	}
	if c.Sitemap != nil && c.Sitemap.Output == "" {
		c.Sitemap.Output = "sitemap.xml"
	}

	if c.Content.Bundles.Output == "" {
		c.Content.Bundles.Output = "_assets"
	}
	if c.Content.Bundles.Mode == "" {
		c.Content.Bundles.Mode = "fingerprinted"
	}
	switch c.Content.Bundles.Mode {
	case "fingerprinted", "adjacent":
	default:
		return fmt.Errorf("content.bundles.mode must be \"fingerprinted\" or \"adjacent\" (got %q)", c.Content.Bundles.Mode)
	}

	if _, err := c.OutputPath(); err != nil {
		return err
	}
	if _, err := c.StaticSourcePath(); err != nil {
		return err
	}
	if _, err := c.ContentSourcePath(); err != nil {
		return err
	}
	if _, err := c.TemplateGlob(); err != nil {
		return err
	}

	return nil
}

func (c *Config) WatchedPaths() (paths []string, globs []string, err error) {
	staticPath, err := c.StaticSourcePath()
	if err != nil {
		return nil, nil, err
	}
	contentPath, err := c.ContentSourcePath()
	if err != nil {
		return nil, nil, err
	}
	templateGlob, err := c.TemplateGlob()
	if err != nil {
		return nil, nil, err
	}

	return []string{staticPath, contentPath}, []string{templateGlob}, nil
}

func (c *Config) OutputPath() (string, error) {
	return c.resolvePath("paths.output", c.Paths.Output)
}

func (c *Config) StaticSourcePath() (string, error) {
	return c.resolvePath("paths.static", c.Paths.Static)
}

func (c *Config) ContentSourcePath() (string, error) {
	return c.resolvePath("paths.content", c.Paths.Content)
}

func (c *Config) TemplateGlob() (string, error) {
	return c.resolveGlob("paths.templates", c.Paths.Templates)
}

func (c *Config) resolvePath(label, raw string) (string, error) {
	root := c.Root
	if root == "" {
		root = "."
	}

	resolved, err := pathutil.ResolvePath(root, raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return resolved.Path, nil
}

func (c *Config) resolveGlob(label, raw string) (string, error) {
	root := c.Root
	if root == "" {
		root = "."
	}

	resolved, err := pathutil.ResolveGlob(root, raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return resolved.Path, nil
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
