package fileutil

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkReturnsSlashRelativeFilesAndDirs(t *testing.T) {
	t.Parallel()

	root := seedWalkTree(t)

	files, dirs, err := Walk(root)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	wantFiles := []string{"assets/logo.png", "index.html", "posts/hello.md"}
	wantDirs := []string{".", "assets", "posts"}

	if got := sortedSetValues(files); !slices.Equal(got, wantFiles) {
		t.Fatalf("Walk() files = %#v, want %#v", got, wantFiles)
	}
	if got := sortedSetValues(dirs); !slices.Equal(got, wantDirs) {
		t.Fatalf("Walk() dirs = %#v, want %#v", got, wantDirs)
	}
}

func TestWalkFilesReturnsOnlyFiles(t *testing.T) {
	t.Parallel()

	root := seedWalkTree(t)

	files, err := WalkFiles(root)
	if err != nil {
		t.Fatalf("WalkFiles() error = %v", err)
	}

	want := []string{"assets/logo.png", "index.html", "posts/hello.md"}
	if got := sortedSetValues(files); !slices.Equal(got, want) {
		t.Fatalf("WalkFiles() = %#v, want %#v", got, want)
	}
}

func TestWalkDirsReturnsRootAndNestedDirs(t *testing.T) {
	t.Parallel()

	root := seedWalkTree(t)

	dirs, err := WalkDirs(root)
	if err != nil {
		t.Fatalf("WalkDirs() error = %v", err)
	}

	want := []string{".", "assets", "posts"}
	if got := sortedSetValues(dirs); !slices.Equal(got, want) {
		t.Fatalf("WalkDirs() = %#v, want %#v", got, want)
	}
}

func TestWalkInfoSplitsFilesAndDirs(t *testing.T) {
	t.Parallel()

	root := seedWalkTree(t)

	files, dirs, err := WalkInfo(root)
	if err != nil {
		t.Fatalf("WalkInfo() error = %v", err)
	}

	for _, rel := range []string{"assets/logo.png", "index.html", "posts/hello.md"} {
		info, ok := files[rel]
		if !ok {
			t.Fatalf("WalkInfo() missing file %q", rel)
		}
		if info.IsDir() {
			t.Fatalf("WalkInfo() file %q reported as directory", rel)
		}
	}
	for _, rel := range []string{".", "assets", "posts"} {
		info, ok := dirs[rel]
		if !ok {
			t.Fatalf("WalkInfo() missing dir %q", rel)
		}
		if !info.IsDir() {
			t.Fatalf("WalkInfo() dir %q reported as file", rel)
		}
	}
}

func seedWalkTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"assets", "posts"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("create dir %q: %v", dir, err)
		}
	}
	for _, rel := range []string{"index.html", "assets/logo.png", "posts/hello.md"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(rel), 0o644); err != nil {
			t.Fatalf("write file %q: %v", rel, err)
		}
	}
	return root
}

func sortedSetValues[T ~string](values interface{ Values() []T }) []T {
	out := values.Values()
	slices.Sort(out)
	return out
}
