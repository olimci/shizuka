package build

import (
	"time"

	"github.com/olimci/shizuka/internal/transforms"
	"github.com/olimci/shizuka/internal/utils/fileutil"
	"github.com/olimci/shizuka/internal/utils/gitutil"
)

const (
	gitTTL = time.Minute
)

type fileFingerprint = fileutil.Stat

type gitFileCacheEntry struct {
	Fingerprint fileFingerprint
	ExpiresAt   time.Time
	Info        *transforms.PageGitMeta
}

type gitStepCache struct {
	Unavailable bool
	Repo        *gitutil.Repo
	Site        *transforms.SiteGitMeta
	SiteExpires time.Time
	Files       map[string]gitFileCacheEntry
}
