package gitutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/transforms"
)

var ErrUnavailable = errors.New("git metadata unavailable")

type Repo struct {
	root   string
	gitDir string
}

func Open(ctx context.Context, startDir string) (*Repo, error) {
	if startDir == "" {
		return nil, fmt.Errorf("%w: empty start dir", ErrUnavailable)
	}

	startDir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("%w: abs path: %w", ErrUnavailable, err)
	}

	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("%w: git executable not found", ErrUnavailable)
	}

	root, err := git(ctx, startDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	gitDir, err := git(ctx, startDir, "rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}

	return &Repo{
		root:   root,
		gitDir: filepath.Clean(gitDir),
	}, nil
}

func (r *Repo) Repo(ctx context.Context) (*transforms.SiteGitMeta, error) {
	head, err := git(ctx, r.root, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	short, err := git(ctx, r.root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return nil, err
	}
	branch, err := git(ctx, r.root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	status, err := git(ctx, r.root, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	return &transforms.SiteGitMeta{
		Available:  true,
		Branch:     branch,
		CommitHash: head,
		ShortHash:  short,
		Dirty:      status != "",
	}, nil
}

func (r *Repo) File(ctx context.Context, relPath string, followRenames bool) (*transforms.PageGitMeta, error) {
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
		return &transforms.PageGitMeta{}, nil
	}

	latestLines, err := log(ctx, r.root, relPath, "-1", "--format=%H%x00%h%x00%an%x00%aI")
	if err != nil {
		return nil, err
	}
	if len(latestLines) == 0 {
		return &transforms.PageGitMeta{}, nil
	}

	parts := strings.Split(latestLines[0], "\x00")
	if len(parts) != 4 {
		return nil, fmt.Errorf("unexpected git log format for %q", relPath)
	}

	updated, err := time.Parse(time.RFC3339, parts[3])
	if err != nil {
		return nil, fmt.Errorf("parse updated time for %q: %w", relPath, err)
	}

	args := []string{"--format=%aI"}
	if followRenames {
		args = append(args, "--follow")
	}
	args = append(args, "--reverse")

	createdLines, err := log(ctx, r.root, relPath, args...)
	if err != nil {
		return nil, err
	}

	var created time.Time
	if len(createdLines) > 0 {
		created, err = time.Parse(time.RFC3339, createdLines[0])
		if err != nil {
			return nil, fmt.Errorf("parse created time for %q: %w", relPath, err)
		}
	}

	return &transforms.PageGitMeta{
		Tracked:    true,
		Created:    created,
		Updated:    updated,
		CommitHash: parts[0],
		ShortHash:  parts[1],
		AuthorName: parts[2],
	}, nil
}

func log(ctx context.Context, root, relPath string, args ...string) ([]string, error) {
	fullArgs := append([]string{"log"}, args...)
	fullArgs = append(fullArgs, "--", relPath)

	out, err := git(ctx, root, fullArgs...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	lines := strings.Split(out, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		if line == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)

	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}

	return strings.TrimSpace(stdout.String()), nil
}
