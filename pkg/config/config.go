package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/olimci/shizuka/pkg/version"

	"github.com/BurntSushi/toml"

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
}

type ConfigShizuka struct {
	Version string `toml:"version"`
}

type ConfigSite struct {
	Title       string `toml:"title"`
	Description string `toml:"description"`
	URL         string `toml:"url"`
}

type ConfigBuild struct {
	Output string `toml:"output"`
	Minify bool   `toml:"minify"`

	Steps ConfigBuildSteps `toml:"steps"`
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
	TemplateGlob      string         `toml:"template_glob"`
	Source            string         `toml:"source"`
	Destination       string         `toml:"destination"`
	DefaultParams     map[string]any `toml:"default_params"`
	DefaultLiteParams map[string]any `toml:"default_lite_params"`
	GoldmarkConfig    ConfigGoldmark `toml:"goldmark_config"`
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
					TemplateGlob:      "templates/*.tmpl",
					Source:            "content",
					Destination:       ".",
					DefaultParams:     map[string]any{},
					DefaultLiteParams: map[string]any{},
					GoldmarkConfig:    defaultGoldmark,
				},
			},
		},
	}
}

// Load loads a Config from a file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	md, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, err
	}

	if undec := md.Undecoded(); len(undec) > 0 {
		return nil, fmt.Errorf("unknown config keys: %v", undec)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate validates the Config.
func (c *Config) Validate() error {
	c.Site.URL = strings.TrimSpace(c.Site.URL)
	if c.Site.URL == "" {
		return errors.New("site.url is required")
	}
	if !(strings.HasPrefix(c.Site.URL, "http://") || strings.HasPrefix(c.Site.URL, "https://")) {
		return fmt.Errorf("site.url must start with http:// or https:// (got %q)", c.Site.URL)
	}
	if _, err := url.Parse(c.Site.URL); err != nil {
		return fmt.Errorf("site.url is not a valid URL (got %q): %w", c.Site.URL, err)
	}

	if strings.TrimSpace(c.Build.Output) == "" {
		c.Build.Output = "dist"
	}

	if c.Build.Steps.Static != nil {
		if strings.TrimSpace(c.Build.Steps.Static.Source) == "" {
			c.Build.Steps.Static.Source = "static"
		}
		if strings.TrimSpace(c.Build.Steps.Static.Destination) == "" {
			c.Build.Steps.Static.Destination = "."
		}
	}

	if c.Build.Steps.Content != nil {
		if strings.TrimSpace(c.Build.Steps.Content.TemplateGlob) == "" {
			c.Build.Steps.Content.TemplateGlob = "templates/*.tmpl"
		}
		if strings.TrimSpace(c.Build.Steps.Content.Source) == "" {
			c.Build.Steps.Content.Source = "content"
		}
		if strings.TrimSpace(c.Build.Steps.Content.Destination) == "" {
			c.Build.Steps.Content.Destination = "."
		}
		if c.Build.Steps.Content.DefaultParams == nil {
			c.Build.Steps.Content.DefaultParams = map[string]any{}
		}
		if c.Build.Steps.Content.DefaultLiteParams == nil {
			c.Build.Steps.Content.DefaultLiteParams = map[string]any{}
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
		shorten := strings.TrimSpace(c.Build.Steps.Redirects.Shorten)
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
		if strings.TrimSpace(c.Build.Steps.RSS.Output) == "" {
			c.Build.Steps.RSS.Output = "rss.xml"
		}
	}

	if c.Build.Steps.Sitemap != nil {
		if strings.TrimSpace(c.Build.Steps.Sitemap.Output) == "" {
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
		switch strings.ToLower(strings.TrimSpace(name)) {
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
