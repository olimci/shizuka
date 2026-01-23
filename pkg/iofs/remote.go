package iofs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sync"
)

// RemoteSource clones a git repository to a temporary directory.
func FromRemote(url string) *RemoteFS {
	return &RemoteFS{url: url}
}

type RemoteFS struct {
	url     string
	tempDir string
	once    sync.Once
	err     error
}

func (r *RemoteFS) FS(ctx context.Context) (fs.FS, error) {
	r.once.Do(func() {
		r.tempDir, r.err = r.clone(ctx)
	})

	if r.err != nil {
		return nil, r.err
	}

	return os.DirFS(r.tempDir), nil
}

func (r *RemoteFS) Root() string {
	return "."
}

func (r *RemoteFS) Close() error {
	if r.tempDir != "" {
		return os.RemoveAll(r.tempDir)
	}
	return nil
}

// clone performs a shallow git clone of the repository.
func (r *RemoteFS) clone(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is required for remote sources: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "shizuka-source-*")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", r.url, tempDir)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("cloning repository: %w", err)
	}

	return tempDir, nil
}
