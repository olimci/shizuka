package transforms

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/olimci/shizuka/pkg/frontmatter"
	"github.com/olimci/shizuka/pkg/utils/decodeutil"
)

var ErrUnsupportedContentType = errors.New("unsupported content type")

type dataPage struct {
	frontmatter.Frontmatter `toml:",inline" yaml:",inline" json:",inline"`
	Body                    string `toml:"body" yaml:"body" json:"body"`
	BodyMarkdown            bool   `toml:"body_markdown" yaml:"body_markdown" json:"body_markdown"`
}

func BuildPage(sourceFS fs.FS, source string, defaultSection string, defaults frontmatter.Defaults, bySection map[string]frontmatter.Defaults) (*Page, error) {
	source = path.Clean(source)
	doc, err := fs.ReadFile(sourceFS, source)
	if err != nil {
		return nil, err
	}

	var (
		meta       frontmatter.Frontmatter
		body       []byte
		preprocess string
	)

	switch ext := strings.ToLower(path.Ext(source)); ext {
	case ".md", ".html":
		fm, extractedBody, err := frontmatter.ExtractWithDefaults(doc, defaultSection, defaults, bySection)
		if err != nil {
			return nil, err
		}
		meta = *fm
		body = extractedBody

		if ext == ".md" {
			preprocess = "markdown"
		}

	default:
		format, ok := decodeutil.FormatExt(ext)
		if !ok {
			return nil, fmt.Errorf("%w %q", ErrUnsupportedContentType, ext)
		}

		base, err := frontmatter.BaseForData(format, doc, defaultSection, defaults, bySection)
		if err != nil {
			return nil, err
		}
		dp := dataPage{Frontmatter: base}
		if err := decodeutil.UnmarshalExt(ext, doc, &dp); err != nil {
			return nil, err
		}
		meta = dp.Frontmatter
		body = []byte(dp.Body)

		if dp.BodyMarkdown {
			preprocess = "markdown"
		}
	}

	page := &Page{
		SourcePath: source,
		Preprocess: preprocess,
		RawBody:    string(body),
	}
	if preprocess == "" {
		page.Body = template.HTML(body)
	}

	page.ApplyFrontmatter(meta)
	return page, nil
}
