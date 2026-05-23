package build

import (
	"time"

	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
	"github.com/olimci/shizuka/pkg/utils/gitutil"
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
