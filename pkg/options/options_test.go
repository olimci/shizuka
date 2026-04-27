package options

import (
	"context"
	"errors"
	"html/template"
	"testing"
)

func TestFilterAndIf(t *testing.T) {
	opt := WithDev(true)
	got := Filter(nil, If(opt, true), If(WithSyncWrites(true), false))

	if len(got) != 1 {
		t.Fatalf("len(Filter(...)) = %d, want 1", len(got))
	}
}

func TestApplyUsesDefaultsAndCopiesMutableInputs(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "value")
	changed := []string{"content/index.md"}
	errKey := errors.New("page")
	errTmpl := template.Must(template.New("err").Parse("error"))
	pageTemplates := map[error]*template.Template{errKey: errTmpl}

	opts := (*Options)(nil).Apply(
		WithContext(ctx),
		WithConfigPath("custom.toml"),
		WithChangedPaths(changed),
		WithPageErrTemplates(pageTemplates),
	)

	changed[0] = "mutated"
	delete(pageTemplates, errKey)

	if opts.Context != ctx {
		t.Fatal("Apply() did not set context")
	}
	if opts.ConfigPath != "custom.toml" {
		t.Fatalf("opts.ConfigPath = %q, want %q", opts.ConfigPath, "custom.toml")
	}
	if opts.MaxWorkers <= 0 {
		t.Fatalf("opts.MaxWorkers = %d, want > 0", opts.MaxWorkers)
	}
	if len(opts.ChangedPaths) != 1 || opts.ChangedPaths[0] != "content/index.md" {
		t.Fatalf("opts.ChangedPaths = %#v, want copied input", opts.ChangedPaths)
	}
	if len(opts.PageErrTemplates) != 1 || opts.PageErrTemplates[errKey] != errTmpl {
		t.Fatalf("opts.PageErrTemplates = %#v, want copied map", opts.PageErrTemplates)
	}
}

func TestDefaultOptionsAndApplyToExisting(t *testing.T) {
	opts := DefaultOptions().Apply(
		WithOutputPath("dist-custom"),
		WithSiteURL("https://example.com"),
		WithSkipOutputCleanup(true),
	)

	if opts.ConfigPath != "shizuka.toml" {
		t.Fatalf("opts.ConfigPath = %q, want %q", opts.ConfigPath, "shizuka.toml")
	}
	if opts.OutputPath != "dist-custom" {
		t.Fatalf("opts.OutputPath = %q, want %q", opts.OutputPath, "dist-custom")
	}
	if opts.SiteURL != "https://example.com" {
		t.Fatalf("opts.SiteURL = %q, want https://example.com", opts.SiteURL)
	}
	if !opts.SkipOutputCleanup {
		t.Fatal("opts.SkipOutputCleanup = false, want true")
	}
}
