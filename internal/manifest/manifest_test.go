package manifest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/options"
)

func TestManifestStartRejectsNonEmptyOutputWithoutForce(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "stale.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := New().Start(context.Background(), manifestTestConfig(root), options.DefaultOptions(), nil, out)
	if err == nil {
		t.Fatal("expected non-empty output to be rejected")
	}
	if !strings.Contains(err.Error(), "pass --force") {
		t.Fatalf("expected force hint, got %v", err)
	}
}

func TestManifestForceReconcilesOutput(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(out, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := options.DefaultOptions().Apply(options.WithForce(true))
	man := New()
	if err := man.Start(context.Background(), manifestTestConfig(root), opts, nil, out); err != nil {
		t.Fatal(err)
	}
	if err := man.Emit(TextArtefact(NewInternalClaim("test", "index.html"), "ok")); err != nil {
		t.Fatal(err)
	}
	if err := man.Finish(true); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ok" {
		t.Fatalf("index.html = %q, want ok", got)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale file stat error = %v, want not exist", err)
	}
}

func manifestTestConfig(root string) *config.Config {
	return &config.Config{
		Root: root,
		Paths: config.ConfigPaths{
			Output:    "dist",
			Content:   "content",
			Data:      "data",
			Static:    "static",
			Templates: "templates",
		},
	}
}
