package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
)

func TestStepRedirectsOrdersGeneratedRulesBeforeConfigSplats(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()
	cfg.Redirects = &config.ConfigRedirects{
		Output:  "_redirects",
		Shorten: "/s",
		Entries: []config.Redirect{
			{From: "/posts/*", To: "/archive/:splat", Status: 301},
			{From: "/legacy", To: "/new", Status: 302},
		},
	}

	out := t.TempDir()
	man := manifest.New()
	opts := options.DefaultOptions()
	opts.OutputPath = out
	opts.ArtefactWorkers = 0
	if err := man.Begin(context.Background(), cfg, opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	reg := registry.New()
	registry.SetAs(reg, PagesK, []*transforms.Page{
		{
			SourcePath: "content/posts/hello.md",
			URLPath:    "posts/hello",
			Slug:       "posts/hello",
			Section:    "posts",
			Aliases:    []string{"posts/latest"},
		},
	})

	step := StepRedirects(cfg)
	sc := &StepContext{
		Manifest: man,
		Registry: reg,
		errors:   &errorState{},
	}
	if err := step.Fn(context.Background(), sc); err != nil {
		t.Fatalf("StepRedirects failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "_redirects"))
	if err != nil {
		t.Fatalf("read _redirects: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{
		"/s/hello /posts/hello",
		"/posts/latest /posts/hello 301",
		"/legacy /new 302",
		"/posts/* /archive/:splat 301",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("_redirects = %#v, want %#v", lines, want)
	}
}

func TestStepRedirectsCanDisableGeneratedShortLinks(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()
	cfg.Redirects = &config.ConfigRedirects{
		Output:            "_redirects",
		Shorten:           "/s",
		DisableShortLinks: true,
	}

	out := t.TempDir()
	man := manifest.New()
	opts := options.DefaultOptions()
	opts.OutputPath = out
	opts.ArtefactWorkers = 0
	if err := man.Begin(context.Background(), cfg, opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	reg := registry.New()
	registry.SetAs(reg, BuildCtxK, &BuildCtx{})
	registry.SetAs(reg, PagesK, []*transforms.Page{
		{
			SourcePath: "content/posts/hello.md",
			URLPath:    "posts/hello",
			Slug:       "posts/hello",
			Section:    "posts",
			Aliases:    []string{"posts/latest"},
		},
	})

	step := StepRedirects(cfg)
	sc := &StepContext{
		Manifest: man,
		Registry: reg,
		errors:   &errorState{},
	}
	if err := step.Fn(context.Background(), sc); err != nil {
		t.Fatalf("StepRedirects failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "_redirects"))
	if err != nil {
		t.Fatalf("read _redirects: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "/posts/latest /posts/hello 301"
	if got != want {
		t.Fatalf("_redirects = %q, want %q", got, want)
	}
}

func TestStepRedirectsSkipsDraftGeneratedRulesOutsideDev(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()
	cfg.Redirects = &config.ConfigRedirects{
		Output:  "_redirects",
		Shorten: "/s",
		Entries: []config.Redirect{
			{From: "/legacy", To: "/new", Status: 301},
		},
	}

	out := t.TempDir()
	man := manifest.New()
	opts := options.DefaultOptions()
	opts.OutputPath = out
	opts.ArtefactWorkers = 0
	if err := man.Begin(context.Background(), cfg, opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	reg := registry.New()
	registry.SetAs(reg, BuildCtxK, &BuildCtx{Dev: false})
	registry.SetAs(reg, PagesK, []*transforms.Page{
		{
			SourcePath: "content/posts/draft.md",
			URLPath:    "posts/draft",
			Slug:       "posts/draft",
			Section:    "posts",
			Aliases:    []string{"posts/preview"},
			Draft:      true,
		},
	})

	step := StepRedirects(cfg)
	sc := &StepContext{
		Manifest: man,
		Registry: reg,
		errors:   &errorState{},
	}
	if err := step.Fn(context.Background(), sc); err != nil {
		t.Fatalf("StepRedirects failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "_redirects"))
	if err != nil {
		t.Fatalf("read _redirects: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "/legacy /new 301"
	if got != want {
		t.Fatalf("_redirects = %q, want %q", got, want)
	}
}

func TestStepHeadersSkipsDraftPageHeadersOutsideDev(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()
	cfg.Headers = &config.ConfigHeaders{
		Output: "_headers",
		Values: map[string]map[string]string{
			"/": {"X-Base": "yes"},
		},
	}

	out := t.TempDir()
	man := manifest.New()
	opts := options.DefaultOptions()
	opts.OutputPath = out
	opts.ArtefactWorkers = 0
	if err := man.Begin(context.Background(), cfg, opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	reg := registry.New()
	registry.SetAs(reg, BuildCtxK, &BuildCtx{Dev: false})
	registry.SetAs(reg, PagesK, []*transforms.Page{
		{
			SourcePath: "content/draft.md",
			URLPath:    "draft",
			Draft:      true,
			Headers: map[string]string{
				"X-Draft": "yes",
			},
		},
	})

	step := StepHeaders(cfg)
	sc := &StepContext{
		Manifest: man,
		Registry: reg,
		errors:   &errorState{},
	}
	if err := step.Fn(context.Background(), sc); err != nil {
		t.Fatalf("StepHeaders failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "_headers"))
	if err != nil {
		t.Fatalf("read _headers: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "X-Draft") || strings.Contains(got, "draft") {
		t.Fatalf("_headers included draft data: %q", got)
	}
	if cfg.Headers.Values["draft"] != nil {
		t.Fatalf("StepHeaders mutated config with draft headers: %#v", cfg.Headers.Values)
	}
}
