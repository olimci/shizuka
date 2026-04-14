package config

import (
	"errors"
	"fmt"
	"io/fs"
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

// Config represents the configuration for the build process.
type Config struct {
	Shizuka ConfigShizuka `toml:"shizuka"`
	Site    ConfigSite    `toml:"site"`
	Build   ConfigBuild   `toml:"build"`

	Root     string         `toml:"-"`
	Resolved ConfigResolved `toml:"-"`
}

type ConfigShizuka struct {
	Version string `toml:"version"`
}

type ConfigSite struct {
	Title       string `toml:"title"`
	Description string `toml:"description"`
	URL         string `toml:"url"`

	Params map[string]any `toml:"params"`
}

type ConfigBuild struct {
	Output string `toml:"output"`
	Minify bool   `toml:"minify"`

	Steps ConfigBuildSteps `toml:"steps"`
}

type ConfigResolved struct {
	Build ResolvedBuild `toml:"-"`
}

type ResolvedBuild struct {
	Output string
	Steps  ResolvedBuildSteps
}

type ResolvedBuildSteps struct {
	Static    *ResolvedStepStatic
	Content   *ResolvedStepContent
	Headers   *ResolvedStepOutput
	Redirects *ResolvedStepRedirects
	RSS       *ResolvedStepOutput
	Sitemap   *ResolvedStepOutput
}

type ResolvedStepStatic struct {
	Source      string
	Destination string
}

type ResolvedStepContent struct {
	TemplateGlob string
	Source       string
	Destination  string
}

type ResolvedStepOutput struct {
	Output string
}

type ResolvedStepRedirects struct {
	Output  string
	Shorten string
}

type ConfigBuildSteps struct {
	Static    *ConfigStepStatic    `toml:"static"`
	Content   *ConfigStepContent   `toml:"content"`
	Headers   *ConfigStepHeaders   `toml:"headers"`
	Redirects *ConfigStepRedirects `toml:"redirects"`
	RSS       *ConfigStepRSS       `toml:"rss"`
	Sitemap   *ConfigStepSitemap   `toml:"sitemap"`
}

type ConfigStepStatic struct {
	Source      string `toml:"source"`
	Destination string `toml:"destination"`
}

type ConfigStepContent struct {
	TemplateGlob    string                `toml:"template_glob"`
	Source          string                `toml:"source"`
	Destination     string                `toml:"destination"`
	DefaultTemplate string                `toml:"default_template"`
	DefaultSection  string                `toml:"default_section"`
	DefaultParams   map[string]any        `toml:"default_params"`
	GoldmarkConfig  ConfigGoldmark        `toml:"goldmark_config"`
	Markdown        ConfigContentMarkdown `toml:"markdown"`
	BundleAssets    ConfigContentBundles  `toml:"bundle_assets"`
	Raw             ConfigContentRaw      `toml:"raw"`
	Git             *ConfigStepContentGit `toml:"git"`
}

type ConfigContentMarkdown struct {
	Wikilinks bool `toml:"wikilinks"`
}

type ConfigContentBundles struct {
	Enabled bool   `toml:"enabled"`
	Output  string `toml:"output"`
	Mode    string `toml:"mode"`
}

type ConfigContentRaw struct {
	Markdown bool `toml:"markdown"`
}

type ConfigStepContentGit struct {
	Enabled  bool `toml:"enabled"`
	Backfill bool `toml:"backfill"`
}

type ConfigStepHeaders struct {
	Headers map[string]map[string]string `toml:"headers"`
	Output  string                       `toml:"output"`
}

type ConfigStepRedirects struct {
	Shorten   string     `toml:"shorten"`
	Redirects []Redirect `toml:"redirects"`
	Output    string     `toml:"output"`
}

type ConfigStepRSS struct {
	Output        string   `toml:"output"`
	Sections      []string `toml:"sections"`
	Limit         int      `toml:"limit"`
	IncludeDrafts bool     `toml:"include_drafts"`
}

type ConfigStepSitemap struct {
	Output        string `toml:"output"`
	IncludeDrafts bool   `toml:"include_drafts"`
}

type Redirect struct {
	From   string `toml:"from"`
	To     string `toml:"to"`
	Status int    `toml:"status"`
}

type ConfigGoldmark struct {
	Extensions []string               `toml:"extensions"`
	Parser     ConfigGoldmarkParser   `toml:"parser"`
	Renderer   ConfigGoldmarkRenderer `toml:"renderer"`
}

type ConfigGoldmarkParser struct {
	AutoHeadingID bool `toml:"auto_heading_id"`
	Attribute     bool `toml:"attribute"`
}

type ConfigGoldmarkRenderer struct {
	Hardbreaks bool `toml:"hardbreaks"`
	XHTML      bool `toml:"XHTML"`
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
	cfg.Root = filepath.Dir(filepath.Clean(path))
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if err := cfg.resolvePaths(); err != nil {
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
	cfg.Root = filepath.Dir(filepath.Clean(path))
	if cfg.Root == "" {
		cfg.Root = "."
	}
	if err := cfg.resolvePaths(); err != nil {
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

	if c.Resolved.Build.Steps.Static != nil && c.Resolved.Build.Steps.Static.Source != "" {
		paths = append(paths, c.Resolved.Build.Steps.Static.Source)
	}

	if c.Resolved.Build.Steps.Content != nil {
		if c.Resolved.Build.Steps.Content.Source != "" {
			paths = append(paths, c.Resolved.Build.Steps.Content.Source)
		}
		if c.Resolved.Build.Steps.Content.TemplateGlob != "" {
			globs = append(globs, c.Resolved.Build.Steps.Content.TemplateGlob)
		}
	}

	return paths, globs
}

func (c *Config) resolvePaths() error {
	root := c.Root
	if root == "" {
		root = "."
	}

	output, err := pathutil.ResolvePath(root, c.Build.Output)
	if err != nil {
		return fmt.Errorf("build.output: %w", err)
	}
	c.Resolved.Build.Output = output.Path

	if c.Build.Steps.Static != nil {
		source, err := pathutil.ResolvePath(root, c.Build.Steps.Static.Source)
		if err != nil {
			return fmt.Errorf("build.steps.static.source: %w", err)
		}
		c.Resolved.Build.Steps.Static = &ResolvedStepStatic{
			Source:      source.Path,
			Destination: c.Build.Steps.Static.Destination,
		}
	}

	if c.Build.Steps.Content != nil {
		source, err := pathutil.ResolvePath(root, c.Build.Steps.Content.Source)
		if err != nil {
			return fmt.Errorf("build.steps.content.source: %w", err)
		}
		glob, err := pathutil.ResolveGlob(root, c.Build.Steps.Content.TemplateGlob)
		if err != nil {
			return fmt.Errorf("build.steps.content.template_glob: %w", err)
		}
		c.Resolved.Build.Steps.Content = &ResolvedStepContent{
			Source:       source.Path,
			TemplateGlob: glob.Path,
			Destination:  c.Build.Steps.Content.Destination,
		}
	}

	if c.Build.Steps.Headers != nil {
		c.Resolved.Build.Steps.Headers = &ResolvedStepOutput{Output: c.Build.Steps.Headers.Output}
	}
	if c.Build.Steps.Redirects != nil {
		c.Resolved.Build.Steps.Redirects = &ResolvedStepRedirects{
			Output:  c.Build.Steps.Redirects.Output,
			Shorten: c.Build.Steps.Redirects.Shorten,
		}
	}
	if c.Build.Steps.RSS != nil {
		c.Resolved.Build.Steps.RSS = &ResolvedStepOutput{Output: c.Build.Steps.RSS.Output}
	}
	if c.Build.Steps.Sitemap != nil {
		c.Resolved.Build.Steps.Sitemap = &ResolvedStepOutput{Output: c.Build.Steps.Sitemap.Output}
	}

	return nil
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
