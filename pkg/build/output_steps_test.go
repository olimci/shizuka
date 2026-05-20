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
