package markdown

import (
	"errors"
	"strings"
	"testing"

	"github.com/olimci/shizuka/internal/config"
)

func TestRenderWikilinks(t *testing.T) {
	md := Build(config.ConfigContentMarkdown{Wikilinks: true}, Options{
		TargetResolver: func(target string) (string, error) {
			if target != "Guide" {
				t.Fatalf("target = %q, want Guide", target)
			}
			return "/guide/?q=one&x=<two>", nil
		},
	})

	doc, err := Render(md, "test.md", "See [[Guide|the guide]].")
	if err != nil {
		t.Fatal(err)
	}

	body := string(doc.Body)
	if !strings.Contains(body, `<a href="/guide/?q=one&amp;x=%3Ctwo%3E">the guide</a>`) {
		t.Fatalf("body missing rendered wikilink:\n%s", body)
	}
}

func TestRenderWikilinkPropagatesResolverError(t *testing.T) {
	md := Build(config.ConfigContentMarkdown{Wikilinks: true}, Options{
		TargetResolver: func(target string) (string, error) {
			return "", errors.New("missing target")
		},
	})

	if _, err := Render(md, "test.md", "[[Missing]]"); err == nil || !strings.Contains(err.Error(), `wikilink "Missing": missing target`) {
		t.Fatalf("err = %v, want wikilink resolver error", err)
	}
}
