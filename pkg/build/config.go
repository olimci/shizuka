package build

import (
	"errors"
	"fmt"
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
}

type ConfigStepRedirects struct {
	Shorten   string     `toml:"shorten"`
	Redirects []Redirect `toml:"redirects"`
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

// LoadConfig loads a Config from a file.
func LoadConfig(path string) (*Config, error) {
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
	}

	return nil
}

// makeGoldmark constructs a new Goldmark instance with the given configuration.
func makeGoldmark(cfg ConfigGoldmark) gm.Markdown {
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
