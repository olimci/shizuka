package manifest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
)

func TestBuildSkipsOutputCleanupWhenRequested(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	stale := filepath.Join(out, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	man := New()
	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	opts := options.DefaultOptions()
	opts.SkipOutputCleanup = true

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out, nil); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("expected stale file to remain, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatalf("expected output file: %v", err)
	}
}

func TestBuildCleansOutputWhenEnabled(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	stale := filepath.Join(out, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	man := New()
	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out, nil); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removed, got %v", err)
	}
}

func TestBeginWritesArtefactsEagerly(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out, nil); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	man.Emit(Artefact{
		Claim: Claim{Owner: "test", Target: "index.html"},
		Builder: func(w io.Writer) error {
			_, err := w.Write([]byte("hello"))
			return err
		},
	})

	target := filepath.Join(out, "index.html")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(target); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected eager output %q to be written before Complete", target)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestBeginReportsDuplicateClaims(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out, nil); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	first := TextArtefact(Claim{Owner: "one", Target: "index.html"}, "one")
	second := TextArtefact(Claim{Owner: "two", Target: "index.html"}, "two")
	man.Emit(first)
	man.Emit(second)

	if err := man.Complete(true); err == nil {
		t.Fatalf("expected duplicate claim failure")
	}
}
