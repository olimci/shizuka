package markdown

import (
	"errors"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/olimci/shizuka/internal/config"
	gm "github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	gmext "github.com/yuin/goldmark/extension"
	gmparse "github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

type TargetResolver func(target string) (string, error)

type Options struct {
	TargetResolver TargetResolver
}

var ErrTargetResolverRequired = errors.New("target resolver required")

func Build(cfg config.ConfigContentMarkdown, options Options) gm.Markdown {
	var (
		exts       []gm.Extender
		parserOpts []gmparse.Option
		htmlOpts   []gmrenderer.Option
	)

	if cfg.Tables {
		exts = append(exts, gmext.Table)
	}
	if cfg.Strikethrough {
		exts = append(exts, gmext.Strikethrough)
	}
	if cfg.TaskList {
		exts = append(exts, gmext.TaskList)
	}
	if cfg.Linkify {
		exts = append(exts, gmext.Linkify)
	}
	if cfg.Typographer {
		exts = append(exts, gmext.Typographer)
	}
	if cfg.DefinitionList {
		exts = append(exts, gmext.DefinitionList)
	}
	if cfg.Footnotes {
		exts = append(exts, gmext.Footnote)
	}
	if cfg.Wikilinks {
		if options.TargetResolver == nil {
			panic(ErrTargetResolverRequired)
		}
		exts = append(exts, newWikilinkExtension(options.TargetResolver))
	}
	if cfg.Highlighting != nil {
		highlightOpts := []highlighting.Option{
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(cfg.Highlighting.LineNumbers),
			),
		}
		if cfg.Highlighting.Style != "" {
			highlightOpts = append(highlightOpts, highlighting.WithStyle(cfg.Highlighting.Style))
		}
		exts = append(exts, highlighting.NewHighlighting(highlightOpts...))
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
