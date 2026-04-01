package build

import (
	"context"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
)

func TestResolveStepKeepsCanonicalOnPageURL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Site.URL = "https://example.com"
	cfg.Build.Steps.Redirects = &config.ConfigStepRedirects{
		Shorten: "/s",
		Output:  "_redirects",
	}

	page := &transforms.Page{
		Meta: transforms.PageMeta{
			Source:  "content/posts/hello.md",
			URLPath: "posts/hello",
		},
		Slug:    "posts/hello",
		Section: "posts",
	}

	node := &transforms.PageNode{
		Page:    page,
		Path:    page.Meta.Source,
		URLPath: page.Meta.URLPath,
	}
	page.Tree = node

	pages := transforms.NewPageTree(&transforms.PageNode{
		Path: ".",
		Children: map[string]*transforms.PageNode{
			"hello": node,
		},
	})

	man := manifest.New()
	man.Set(string(OptionsK), config.DefaultOptions())
	man.Set(string(ConfigK), cfg)
	man.Set(string(PagesK), pages)
	man.Set(string(BuildCtxK), &BuildCtx{
		StartTime:       time.Unix(0, 0),
		StartTimestring: time.Unix(0, 0).String(),
	})

	sc := &StepContext{
		Ctx:      context.Background(),
		Manifest: man,
		errors:   &errorState{},
	}

	resolve := StepContent()[2]
	if err := resolve.Fn(sc); err != nil {
		t.Fatalf("resolve step: %v", err)
	}

	const want = "https://example.com/posts/hello/"
	if page.Canon != want {
		t.Fatalf("page canon = %q, want %q", page.Canon, want)
	}

	site := manifest.GetAs(man, SiteK)
	if site == nil {
		t.Fatal("site was not registered")
	}
	if got := site.Collections.All[0].Canon; got != want {
		t.Fatalf("collection canon = %q, want %q", got, want)
	}
}

func TestRedirectsStepReportsDuplicateShortLinks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Build.Steps.Redirects = &config.ConfigStepRedirects{
		Shorten: "/s",
		Output:  "_redirects",
	}

	first := &transforms.Page{
		Meta: transforms.PageMeta{
			Source:  "content/posts/one.md",
			URLPath: "posts/2024/hello",
		},
		Slug:    "posts/2024/hello",
		Section: "posts",
	}
	second := &transforms.Page{
		Meta: transforms.PageMeta{
			Source:  "content/posts/two.md",
			URLPath: "posts/2025/hello",
		},
		Slug:    "posts/2025/hello",
		Section: "posts",
	}

	pages := transforms.NewPageTree(&transforms.PageNode{
		Path: ".",
		Children: map[string]*transforms.PageNode{
			"one": {Page: first},
			"two": {Page: second},
		},
	})

	sc := &StepContext{
		Ctx: context.Background(),
		Manifest: func() *manifest.Manifest {
			man := manifest.New()
			man.Set(string(ConfigK), cfg)
			man.Set(string(PagesK), pages)
			return man
		}(),
		errors: &errorState{},
	}

	if err := StepRedirects().Fn(sc); err != nil {
		t.Fatalf("redirects step: %v", err)
	}

	if !sc.errors.HasErrors() {
		t.Fatal("expected duplicate short redirect error")
	}

	errs := sc.errors.Slice()
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}
	if got := errs[0].Description(); got != `duplicate redirect "/s/hello" (/posts/2024/hello, /posts/2025/hello)` {
		t.Fatalf("error = %q", got)
	}
}
