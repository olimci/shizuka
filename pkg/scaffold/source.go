package scaffold

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sync"
)

// Source represents an arbitrary Source for a scaffold
type Source interface {
	FS(context.Context) (fs.FS, error)
	Root() string
	Close() error
}

// FSSource wraps any io/fs.FS
func NewFSSource(fsys fs.FS, root string) *FSSource {
	return &FSSource{fs: fsys, root: root}
}

type FSSource struct {
	fs   fs.FS
	root string
}

func (f *FSSource) FS(ctx context.Context) (fs.FS, error) {
	return f.fs, nil
}

func (f *FSSource) Root() string {
	return f.root
}

func (f *FSSource) Close() error {
	return nil
}

// OSSource wraps any path with a os.DirFS
func NewOSSource(path string) *OSSource {
	return &OSSource{path: path}
}

type OSSource struct {
	path string
}

func (o *OSSource) FS(ctx context.Context) (fs.FS, error) {
	return os.DirFS(o.path), nil
}

func (o *OSSource) Root() string {
	return "."
}

func (o *OSSource) Close() error {
	return nil
}

// RemoteSource clones a git repository to a temporary directory
func NewRemoteSource(url string) *RemoteSource {
	return &RemoteSource{url: url}
}

type RemoteSource struct {
	url     string
	tempDir string
	once    sync.Once
	err     error
}

func (r *RemoteSource) FS(ctx context.Context) (fs.FS, error) {
	r.once.Do(func() {
		r.tempDir, r.err = r.clone(ctx)
	})

	if r.err != nil {
		return nil, r.err
	}

	return os.DirFS(r.tempDir), nil
}

func (r *RemoteSource) Root() string {
	return "."
}

func (r *RemoteSource) Close() error {
	if r.tempDir != "" {
		return os.RemoveAll(r.tempDir)
	}
	return nil
}

// clone performs a shallow git clone of the repository
func (r *RemoteSource) clone(ctx context.Context) (string, error) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is required for remote sources: %w", err)
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "shizuka-scaffold-*")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	// Shallow clone for speed
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", r.url, tempDir)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Clean up on failure
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("cloning repository: %w", err)
	}

	return tempDir, nil
}
