package fileutil

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicEditWithOptionsSkipsIdenticalContent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	changed, err := AtomicEditWithOptions(path, func(w io.Writer) error {
		_, err := w.Write([]byte("same"))
		return err
	}, AtomicOptions{})
	if err != nil {
		t.Fatalf("AtomicEditWithOptions failed: %v", err)
	}
	if changed {
		t.Fatalf("expected unchanged edit to be skipped")
	}
}

func TestAtomicWriteWithOptionsWritesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "index.html")
	changed, err := AtomicWriteWithOptions(path, func(w io.Writer) error {
		_, err := w.Write([]byte("hello"))
		return err
	}, AtomicOptions{})
	if err != nil {
		t.Fatalf("AtomicWriteWithOptions failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected write to report changed")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("unexpected content %q", got)
	}
}
