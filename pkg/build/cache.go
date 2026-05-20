package build

import (
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/git"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
)

const (
	gitTTL = time.Minute
)

type fileFingerprint = fileutil.Stat

func statFingerprint(path string) (fileFingerprint, error) {
	return fileutil.Info(path)
}

type staticFileCacheEntry struct {
	Fingerprint fileFingerprint
	Claim       manifest.Claim
}

type staticStepCache struct {
	Files map[string]staticFileCacheEntry
}

type pageIndexCacheEntry struct {
	Fingerprint fileFingerprint
	Page        *transforms.Page
}

type pageIndexStepCache struct {
	ContentFiles []string
	Entries      map[string]pageIndexCacheEntry
}

type pageAssetsCacheEntry struct {
	Inputs map[string]fileFingerprint
	Assets map[string]*transforms.PageAsset
}

type pageAssetsStepCache struct {
	Entries map[string]pageAssetsCacheEntry
}

type gitFileCacheEntry struct {
	Fingerprint fileFingerprint
	ExpiresAt   time.Time
	Info        *transforms.PageGitMeta
}

type gitStepCache struct {
	Unavailable bool
	Repo        *git.Repo
	Site        *transforms.SiteGitMeta
	SiteExpires time.Time
	Files       map[string]gitFileCacheEntry
}

func normalizeChangedPaths(paths []string) []string {
	if paths == nil {
		return nil
	}

	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		abs, err := filepath.Abs(raw)
		if err != nil {
			out = append(out, filepath.Clean(raw))
			continue
		}
		out = append(out, filepath.Clean(abs))
	}

	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) == 0 {
		return []string{}
	}
	return out
}
