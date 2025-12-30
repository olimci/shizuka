package build

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/olimci/shizuka/pkg/version"

	"github.com/BurntSushi/toml"

	gm "github.com/yuin/goldmark"
	gmext "github.com/yuin/goldmark/extension"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

type Config struct {
	Shizuka ShizukaConfig `toml:"shizuka"`
	Site    SiteConfig    `toml:"site"`
	Content ContentConfig `toml:"content"`
	Build   BuildConfig   `toml:"build"`
}

type ShizukaConfig struct {
	Version string `toml:"version"`
}

type SiteConfig struct {
	Title       string `toml:"title"`
	Description string `toml:"description"`
	URL         string `toml:"url"`
	BasePath    string `toml:"base_path"`
}

type ContentConfig struct {
	DefaultParams     map[string]any `toml:"default_params"`
	DefaultLiteParams map[string]any `toml:"default_lite_params"`
}

type BuildConfig struct {
	OutputDir     string `toml:"output_dir"`
	TemplatesGlob string `toml:"templates_glob"`
	StaticDir     string `toml:"static_dir"`
	ContentDir    string `toml:"content_dir"`

	Targets    BuildTargets    `toml:"targets"`
	Transforms BuildTransforms `toml:"transforms"`
	Goldmark   GoldmarkConfig  `toml:"goldmark"`
}

type BuildTargets struct {
	RSS     BuildRSSConfig `toml:"rss"`
	Sitemap BuildSiteMap   `toml:"sitemap"`
}

type BuildRSSConfig struct {
	Enable      bool   `toml:"enable"`
	Path        string `toml:"path"`
	Title       string `toml:"title"`
	Description string `toml:"description"`
}

type BuildSiteMap struct {
	Enable bool   `toml:"enable"`
	Path   string `toml:"path"`
}

type BuildTransforms struct {
	Minify bool `toml:"minify"`
}

type GoldmarkConfig struct {
	Extensions []string         `toml:"extensions"`
	Parser     GoldmarkParser   `toml:"parser"`
	Renderer   GoldmarkRenderer `toml:"renderer"`
}

type GoldmarkParser struct {
	AutoHeadingID bool `toml:"auto_heading_id"`
	Attribute     bool `toml:"attribute"`
}

type GoldmarkRenderer struct {
	Hardbreaks bool `toml:"hardbreaks"`
	XHTML      bool `toml:"XHTML"`
}

func DefaultConfig() *Config {
	return &Config{
		Shizuka: ShizukaConfig{
			Version: version.String(),
		},
		Site: SiteConfig{
			Title:       "Shizuka",
			Description: "Shizuka site",
			URL:         "https://example.com",
			BasePath:    "/",
		},
		Build: BuildConfig{
			Targets: BuildTargets{
				RSS: BuildRSSConfig{
					Enable:      false,
					Path:        "rss.xml",
					Title:       "Shizuka RSS Feed",
					Description: "Shizuka site RSS Feed",
				},
				Sitemap: BuildSiteMap{
					Enable: false,
					Path:   "sitemap.xml",
				},
			},
			Transforms: BuildTransforms{
				Minify: true,
			},
			Goldmark: GoldmarkConfig{
				Extensions: []string{
					"gfm",
					"table",
					"strikethrough",
					"tasklist",
					"deflist",
					"footnotes",
					"typographer",
				},
				Parser: GoldmarkParser{
					AutoHeadingID: false,
					Attribute:     false,
				},
				Renderer: GoldmarkRenderer{
					Hardbreaks: false,
					XHTML:      false,
				},
			},
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	md, err := toml.DecodeFile(path, &cfg)
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

func SaveDefaultConfig(path string) error {
	cfg := DefaultConfig()

	file, err := os.Create(path)
	if err != nil {
		return err
	}

	return toml.NewEncoder(file).Encode(cfg)
}

func (c *Config) Validate() error {
	c.Site.BasePath = strings.TrimSpace(c.Site.BasePath)
	if c.Site.BasePath == "" {
		c.Site.BasePath = "/"
	}
	if !strings.HasPrefix(c.Site.BasePath, "/") {
		c.Site.BasePath = "/" + c.Site.BasePath
	}
	if c.Site.BasePath != "/" && strings.HasSuffix(c.Site.BasePath, "/") {
		c.Site.BasePath = strings.TrimSuffix(c.Site.BasePath, "/")
	}

	c.Site.URL = strings.TrimSpace(c.Site.URL)
	if c.Site.URL == "" {
		return errors.New("site.url is required")
	}
	if !(strings.HasPrefix(c.Site.URL, "http://") || strings.HasPrefix(c.Site.URL, "https://")) {
		return fmt.Errorf("site.url must start with http:// or https:// (got %q)", c.Site.URL)
	}

	if c.Build.Targets.RSS.Enable && strings.TrimSpace(c.Build.Targets.RSS.Path) == "" {
		c.Build.Targets.RSS.Path = "rss.xml"
	}
	if c.Build.Targets.Sitemap.Enable && strings.TrimSpace(c.Build.Targets.Sitemap.Path) == "" {
		c.Build.Targets.Sitemap.Path = "sitemap.xml"
	}

	return nil
}

func MakeGoldmark(cfg GoldmarkConfig) gm.Markdown {
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
