package manifest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/options"
	"github.com/olimci/shizuka/internal/utils/pathutil"
)

func normalizeTarget(dest string) (string, error) {
	rel := path.Clean(filepath.ToSlash(dest))
	if path.IsAbs(rel) || pathutil.EscapesRoot(rel) {
		return "", fmt.Errorf("output path %q escapes the build output root", dest)
	}
	if rel == "." {
		return "", errors.New("output path is empty")
	}
	return rel, nil
}

func validateOutputPath(cfg *config.Config, opts *options.Options, output string) error {
	if cfg == nil {
		return nil
	}

	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("output path %q: %w", output, err)
	}
	rootAbs, err := filepath.Abs(cfg.Root)
	if err != nil {
		return fmt.Errorf("config root %q: %w", cfg.Root, err)
	}

	if opts == nil || !opts.OutputPathInternal {
		rel, err := filepath.Rel(rootAbs, outputAbs)
		if err != nil {
			return fmt.Errorf("output path %q: %w", output, err)
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return fmt.Errorf("output path %q must be a subdirectory of config root %q", output, cfg.Root)
		}
	}

	sourcePaths, err := manifestSourcePaths(cfg)
	if err != nil {
		return err
	}
	for _, source := range sourcePaths {
		if pathsIntersect(outputAbs, source) {
			return fmt.Errorf("output path %q intersects source path %q", outputAbs, source)
		}
	}
	return nil
}

func manifestSourcePaths(cfg *config.Config) ([]string, error) {
	rootAbs, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("config root %q: %w", cfg.Root, err)
	}

	paths := []string{
		filepath.Join(rootAbs, filepath.FromSlash(cfg.Paths.Static)),
		filepath.Join(rootAbs, filepath.FromSlash(cfg.Paths.Content)),
		filepath.Join(rootAbs, filepath.FromSlash(cfg.Paths.Templates)),
	}
	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("source path %q: %w", p, err)
		}
		paths[i] = abs
	}
	return paths, nil
}

func pathsIntersect(a, b string) bool {
	rel, err := filepath.Rel(a, b)
	if err == nil && (rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return true
	}

	rel, err = filepath.Rel(b, a)
	return err == nil && (rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func ensureRootDir(root *os.Root, dir string) error {
	if dir == "" {
		dir = "."
	}
	info, err := root.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	return nil
}

func rootFileExists(root *os.Root, full string) (bool, error) {
	info, err := root.Stat(full)
	if err == nil {
		if info.IsDir() {
			return false, fmt.Errorf("output path %q is a directory", full)
		}
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func rootEmpty(root *os.Root) (bool, error) {
	dir, err := root.Open(".")
	if err != nil {
		return false, err
	}
	defer dir.Close()

	entries, err := dir.ReadDir(1)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return len(entries) == 0, nil
}

func conflictError(target string, claims []Claim) error {
	owners := make([]string, 0, len(claims))
	for _, claim := range claims {
		owners = append(owners, claim.DisplayOwner())
	}
	return fmt.Errorf("%w for %q: claimed by %s", ErrConflicts, target, strings.Join(owners, ", "))
}

// manifestDirs creates a set of directories needed for output files.
func manifestDirs(m map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{})
	for claim := range m {
		claim = path.Clean(filepath.ToSlash(claim))
		if path.IsAbs(claim) || pathutil.EscapesRoot(claim) {
			continue
		}

		dir := path.Dir(claim)
		for dir != "." && dir != "/" {
			out[dir] = struct{}{}
			dir = path.Dir(dir)
		}
	}

	out["."] = struct{}{}

	return out
}
