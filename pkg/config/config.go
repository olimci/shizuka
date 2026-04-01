package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/olimci/shizuka/pkg/version"

	gm "github.com/yuin/goldmark"
	gmext "github.com/yuin/goldmark/extension"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// Config represents the configuration for the build process.
type Config struct {
	Shizuka ConfigShizuka `toml:"shizuka" yaml:"shizuka" json:"shizuka"`
	Site    ConfigSite    `toml:"site" yaml:"site" json:"site"`
	Build   ConfigBuild   `toml:"build" yaml:"build" json:"build"`
}

type ConfigShizuka struct {
	Version string `toml:"version" yaml:"version" json:"version"`
}

type ConfigSite struct {
	Title       string `toml:"title" yaml:"title" json:"title"`
	Description string `toml:"description" yaml:"description" json:"description"`
	URL         string `toml:"url" yaml:"url" json:"url"`

	Params map[string]any `toml:"params" yaml:"params" json:"params"`
}

type ConfigBuild struct {
	Output string `toml:"output" yaml:"output" json:"output"`
	Minify bool   `toml:"minify" yaml:"minify" json:"minify"`

	Steps ConfigBuildSteps `toml:"steps" yaml:"steps" json:"steps"`
}

type ConfigBuildSteps struct {
	Static    *ConfigStepStatic    `toml:"static" yaml:"static" json:"static"`
	Content   *ConfigStepContent   `toml:"content" yaml:"content" json:"content"`
	Headers   *ConfigStepHeaders   `toml:"headers" yaml:"headers" json:"headers"`
	Redirects *ConfigStepRedirects `toml:"redirects" yaml:"redirects" json:"redirects"`
	RSS       *ConfigStepRSS       `toml:"rss" yaml:"rss" json:"rss"`
	Sitemap   *ConfigStepSitemap   `toml:"sitemap" yaml:"sitemap" json:"sitemap"`
}

type ConfigStepStatic struct {
	Source      string `toml:"source" yaml:"source" json:"source"`
	Destination string `toml:"destination" yaml:"destination" json:"destination"`
}

type ConfigStepContent struct {
	TemplateGlob    string                `toml:"template_glob" yaml:"template_glob" json:"template_glob"`
	Source          string                `toml:"source" yaml:"source" json:"source"`
	Destination     string                `toml:"destination" yaml:"destination" json:"destination"`
	DefaultTemplate string                `toml:"default_template" yaml:"default_template" json:"default_template"`
	DefaultParams   map[string]any        `toml:"default_params" yaml:"default_params" json:"default_params"`
	GoldmarkConfig  ConfigGoldmark        `toml:"goldmark_config" yaml:"goldmark_config" json:"goldmark_config"`
	Markdown        ConfigContentMarkdown `toml:"markdown" yaml:"markdown" json:"markdown"`
	BundleAssets    ConfigContentBundles  `toml:"bundle_assets" yaml:"bundle_assets" json:"bundle_assets"`
	Raw             ConfigContentRaw      `toml:"raw" yaml:"raw" json:"raw"`
	Git             *ConfigStepContentGit `toml:"git" yaml:"git" json:"git"`
}

type ConfigContentMarkdown struct {
	ObsidianLinks bool `toml:"obsidian_links" yaml:"obsidian_links" json:"obsidian_links"`
}

type ConfigContentBundles struct {
	Enabled bool   `toml:"enabled" yaml:"enabled" json:"enabled"`
	Output  string `toml:"output" yaml:"output" json:"output"`
	Mode    string `toml:"mode" yaml:"mode" json:"mode"`
}

type ConfigContentRaw struct {
	Markdown bool `toml:"markdown" yaml:"markdown" json:"markdown"`
}

type ConfigStepContentGit struct {
	Enabled  bool `toml:"enabled" yaml:"enabled" json:"enabled"`
	Backfill bool `toml:"backfill" yaml:"backfill" json:"backfill"`
}

type ConfigStepHeaders struct {
	Headers map[string]map[string]string `toml:"headers" yaml:"headers" json:"headers"`
	Output  string                       `toml:"output" yaml:"output" json:"output"`
}

type ConfigStepRedirects struct {
	Shorten   string     `toml:"shorten" yaml:"shorten" json:"shorten"`
	Redirects []Redirect `toml:"redirects" yaml:"redirects" json:"redirects"`
	Output    string     `toml:"output" yaml:"output" json:"output"`
}

type ConfigStepRSS struct {
	Output        string   `toml:"output" yaml:"output" json:"output"`
	Sections      []string `toml:"sections" yaml:"sections" json:"sections"`
	Limit         int      `toml:"limit" yaml:"limit" json:"limit"`
	IncludeDrafts bool     `toml:"include_drafts" yaml:"include_drafts" json:"include_drafts"`
}

type ConfigStepSitemap struct {
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
		Shizuka: ConfigShizuka{
			Version: version.String(),
		},
		Site: ConfigSite{
			Title:       "Shizuka",
			Description: "Shizuka site",
			URL:         "https://example.com",
		},
		Build: ConfigBuild{
			Output: "dist",
			Minify: true,
			Steps: ConfigBuildSteps{
				Static: &ConfigStepStatic{
					Source:      "static",
					Destination: ".",
				},
				Content: &ConfigStepContent{
					TemplateGlob:    "templates/*.tmpl",
					Source:          "content",
					Destination:     ".",
					DefaultTemplate: "page",
					DefaultParams:   map[string]any{},
					GoldmarkConfig:  defaultGoldmark,
					BundleAssets: ConfigContentBundles{
						Enabled: false,
						Output:  "_assets",
						Mode:    "fingerprinted",
					},
				},
			},
		},
	}
}

// Load loads a Config from a file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if err := decodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}
	return cfg, nil
}

// LoadFS loads a Config from a file within the provided fs.FS.
func LoadFS(fsys fs.FS, path string) (*Config, error) {
	cfg := DefaultConfig()

	if err := decodeFS(fsys, path, cfg); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %q: %w", path, err)
	}
	return cfg, nil
}

// Validate validates the Config.
func (c *Config) Validate() error {
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

	if c.Build.Output == "" {
		c.Build.Output = "dist"
	}

	if c.Build.Steps.Static != nil {
		if c.Build.Steps.Static.Source == "" {
			c.Build.Steps.Static.Source = "static"
		}
		if c.Build.Steps.Static.Destination == "" {
			c.Build.Steps.Static.Destination = "."
		}
	}

	if c.Build.Steps.Content != nil {
		if c.Build.Steps.Content.TemplateGlob == "" {
			c.Build.Steps.Content.TemplateGlob = "templates/*.tmpl"
		}
		if c.Build.Steps.Content.Source == "" {
			c.Build.Steps.Content.Source = "content"
		}
		if c.Build.Steps.Content.Destination == "" {
			c.Build.Steps.Content.Destination = "."
		}
		if c.Build.Steps.Content.DefaultTemplate == "" {
			c.Build.Steps.Content.DefaultTemplate = "page"
		}
		if c.Build.Steps.Content.DefaultParams == nil {
			c.Build.Steps.Content.DefaultParams = map[string]any{}
		}
		if c.Build.Steps.Content.BundleAssets.Output == "" {
			c.Build.Steps.Content.BundleAssets.Output = "_assets"
		}
		if c.Build.Steps.Content.BundleAssets.Mode == "" {
			c.Build.Steps.Content.BundleAssets.Mode = "fingerprinted"
		}
		switch c.Build.Steps.Content.BundleAssets.Mode {
		case "fingerprinted", "adjacent":
		default:
			return fmt.Errorf("build.steps.content.bundle_assets.mode must be \"fingerprinted\" or \"adjacent\" (got %q)", c.Build.Steps.Content.BundleAssets.Mode)
		}
	}

	if c.Build.Steps.Headers != nil {
		if c.Build.Steps.Headers.Headers == nil {
			c.Build.Steps.Headers.Headers = make(map[string]map[string]string)
		}
		if c.Build.Steps.Headers.Output == "" {
			c.Build.Steps.Headers.Output = "_headers"
		}
	}

	if c.Build.Steps.Redirects != nil {
		shorten := c.Build.Steps.Redirects.Shorten
		if shorten == "" {
			shorten = "/s"
		}
		if !strings.HasPrefix(shorten, "/") {
			shorten = "/" + shorten
		}
		shorten = strings.TrimSuffix(shorten, "/")
		c.Build.Steps.Redirects.Shorten = shorten

		if c.Build.Steps.Redirects.Output == "" {
			c.Build.Steps.Redirects.Output = "_redirects"
		}
	}

	if c.Build.Steps.RSS != nil {
		if c.Build.Steps.RSS.Output == "" {
			c.Build.Steps.RSS.Output = "rss.xml"
		}
	}

	if c.Build.Steps.Sitemap != nil {
		if c.Build.Steps.Sitemap.Output == "" {
			c.Build.Steps.Sitemap.Output = "sitemap.xml"
		}
	}

	return nil
}

func (c *Config) WatchedPaths() (paths []string, globs []string) {
	paths = make([]string, 0)
	globs = make([]string, 0)

	if c.Build.Steps.Static != nil && c.Build.Steps.Static.Source != "" {
		paths = append(paths, c.Build.Steps.Static.Source)
	}

	if c.Build.Steps.Content != nil {
		if c.Build.Steps.Content.Source != "" {
			paths = append(paths, c.Build.Steps.Content.Source)
		}
		if c.Build.Steps.Content.TemplateGlob != "" {
			globs = append(globs, c.Build.Steps.Content.TemplateGlob)
		}
	}

	return paths, globs
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
