package transforms

import "time"

// PageGitMeta stores git metadata for a page.
type PageGitMeta struct {
	Tracked    bool
	Created    time.Time
	Updated    time.Time
	CommitHash string
	ShortHash  string
	AuthorName string
}

type PageFileMeta struct {
	Available bool
	Created   time.Time
	Updated   time.Time
	Size      int64
}

// SiteGitMeta stores repository metadata for the current build.
type SiteGitMeta struct {
	Available  bool
	RepoRoot   string
	GitDir     string
	Branch     string
	CommitHash string
	ShortHash  string
	Dirty      bool
}
