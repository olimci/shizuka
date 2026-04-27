package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/git"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

func StepStatic(cfg *config.Config) Step {
	return StepFunc("static", func(_ context.Context, sc *StepContext) error {
		staticRoot, err := cfg.StaticSourcePath()
		if err != nil {
			return err
		}

		m := NewMinifier(cfg.Build.Minify)
		cache := registry.GetAs(sc.Cache, StaticCacheK)
		if cache == nil {
			cache = &staticStepCache{Files: make(map[string]staticFileCacheEntry)}
			registry.SetAs(sc.Cache, StaticCacheK, cache)
		}
		if cache.Files == nil {
			cache.Files = make(map[string]staticFileCacheEntry)
		}

		if sc.ChangedPaths != nil && !sc.MayHaveChangesUnder(staticRoot) {
			for _, entry := range cache.Files {
				sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceRoot, entry.Claim).Post(m))
			}
			return nil
		}

		files, err := fileutil.WalkFiles(staticRoot)
		if err != nil {
			return err
		}

		next := make(map[string]staticFileCacheEntry, files.Len())
		for _, rel := range files.Values() {
			abs := filepath.Join(staticRoot, filepath.FromSlash(rel))
			source, err := filepath.Rel(sc.SourceRoot, abs)
			if err != nil {
				return err
			}
			source = filepath.ToSlash(source)

			fingerprint, err := statFingerprint(abs)
			if err != nil {
				return err
			}

			entry, ok := cache.Files[rel]
			if !ok || !entry.Fingerprint.Equal(fingerprint) {
				entry = staticFileCacheEntry{
					Fingerprint: fingerprint,
					Claim: manifest.Claim{
						Owner:  "static",
						Source: source,
						Target: rel,
						Canon:  rel,
					},
				}
			}

			next[rel] = entry
			sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceRoot, entry.Claim).Post(m))
		}

		cache.Files = next
		return nil
	})
}

func StepGit(cfg *config.Config) Step {
	return StepFunc("git", func(ctx context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		gitCfg := cfg.Content.Git
		if gitCfg == nil {
			return nil
		}

		root, err := filepath.Abs(sc.SourceRoot)
		if err != nil {
			return err
		}

		cache := registry.GetAs(sc.Cache, GitCacheK)
		if cache == nil {
			cache = &gitStepCache{Files: make(map[string]gitFileCacheEntry)}
			registry.SetAs(sc.Cache, GitCacheK, cache)
		}
		if cache.Files == nil {
			cache.Files = make(map[string]gitFileCacheEntry)
		}

		repo := cache.Repo
		if repo == nil && !cache.Unavailable {
			repo, err = git.Open(ctx, root)
			if err != nil || repo == nil {
				if errors.Is(err, git.ErrUnavailable) {
					cache.Unavailable = true
					return nil
				}
				return err
			}
			cache.Repo = repo
		}
		if repo == nil {
			return nil
		}

		now := time.Now()
		info := cache.Site
		if info == nil || now.After(cache.SiteExpires) {
			info, err = repo.SiteInfo(ctx)
			if err != nil {
				return err
			}
			cache.Site = info
			cache.SiteExpires = now.Add(gitTTL)
		}
		registry.SetAs(sc.Registry, SiteGitK, info)

		activeFiles := make(map[string]struct{}, len(pages))
		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			abs := filepath.Join(root, filepath.FromSlash(page.SourcePath))
			relPath, err := filepath.Rel(info.RepoRoot, abs)
			if err != nil {
				continue
			}
			relPath = filepath.ToSlash(filepath.Clean(relPath))
			if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
				continue
			}
			activeFiles[relPath] = struct{}{}

			fingerprint, err := statFingerprint(abs)
			if err != nil {
				return err
			}

			pageInfo := (*transforms.PageGitMeta)(nil)
			if entry, ok := cache.Files[relPath]; ok && entry.Fingerprint.Equal(fingerprint) && now.Before(entry.ExpiresAt) && entry.Info != nil {
				pageInfo = entry.Info
			} else {
				pageInfo, err = repo.FileInfo(ctx, relPath, true)
				if err != nil {
					return err
				}
				cache.Files[relPath] = gitFileCacheEntry{
					Fingerprint: fingerprint,
					ExpiresAt:   now.Add(gitTTL),
					Info:        pageInfo,
				}
			}

			applyGitMetadata(page, pageInfo, gitCfg.Backfill)
		}

		for relPath := range cache.Files {
			if _, ok := activeFiles[relPath]; ok {
				continue
			}
			delete(cache.Files, relPath)
		}

		return nil
	}, "pages:index")
}

func applyGitMetadata(page *transforms.Page, info *transforms.PageGitMeta, backfill bool) {
	if info == nil {
		return
	}

	page.Git = *info
	if backfill {
		if !page.Source.HasCreated && !info.Created.IsZero() {
			page.Created = info.Created
		}
		if !page.Source.HasUpdated && !info.Updated.IsZero() {
			page.Updated = info.Updated
		}
	}
	if !page.Updated.IsZero() {
		page.PubDate = page.Updated
	} else if !page.Created.IsZero() {
		page.PubDate = page.Created
	}
}

func StepHeaders(cfg *config.Config) Step {
	return StepFunc("headers", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		headersCfg := cfg.Headers
		if headersCfg == nil {
			return nil
		}

		headers := headersCfg.Values
		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			if len(page.Headers) == 0 {
				continue
			}
			pagePath := page.URLPath
			if _, ok := headers[pagePath]; !ok {
				headers[pagePath] = make(map[string]string, len(page.Headers))
			}
			maps.Copy(headers[pagePath], page.Headers)
		}

		if len(headers) == 0 {
			return nil
		}

		sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("headers", headersCfg.Output),
			Builder: func(w io.Writer) error {
				for pagePath, kvs := range headers {
					fmt.Fprintf(w, "%s\n", pagePath)
					for k, v := range kvs {
						fmt.Fprintf(w, "  %s: %s\n", k, v)
					}
					fmt.Fprintln(w)
				}
				return nil
			},
		})
		return nil
	}, "pages:index")
}

func StepRedirects(cfg *config.Config) Step {
	return StepFunc("redirects", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		redirectsCfg := cfg.Redirects
		if redirectsCfg == nil {
			return nil
		}

		redirects := make([]config.Redirect, 0, len(redirectsCfg.Entries))
		redirects = append(redirects, redirectsCfg.Entries...)

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			if page.Section == "posts" {
				shortSlug := pathutil.ShortSlugForRedirect(page.Slug)
				if shortSlug != "" {
					redirects = append(redirects, config.Redirect{
						From:   path.Join(redirectsCfg.Shorten, shortSlug),
						To:     pathutil.EnsureLeadingSlash(page.URLPath),
						Status: 0,
					})
				}
			}

			for _, alias := range page.Aliases {
				redirects = append(redirects, config.Redirect{
					From:   pathutil.EnsureLeadingSlash(alias),
					To:     pathutil.EnsureLeadingSlash(page.URLPath),
					Status: 301,
				})
			}
		}

		if len(redirects) == 0 {
			return nil
		}

		seen := make(map[string]string, len(redirects))
		deduped := make([]config.Redirect, 0, len(redirects))
		for _, redirect := range redirects {
			if prev, ok := seen[redirect.From]; ok {
				sc.Error(fmt.Errorf("duplicate redirect %q (%s, %s)", redirect.From, prev, redirect.To), manifest.NewInternalClaim("redirects", redirectsCfg.Output))
				continue
			}
			seen[redirect.From] = redirect.To
			deduped = append(deduped, redirect)
		}
		redirects = deduped

		sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("redirects", redirectsCfg.Output),
			Builder: func(w io.Writer) error {
				for _, redirect := range redirects {
					fmt.Fprintf(w, "%s %s", redirect.From, redirect.To)
					if redirect.Status != 0 {
						fmt.Fprintf(w, " %d", redirect.Status)
					}
					fmt.Fprintln(w)
				}
				return nil
			},
		})
		return nil
	}, "pages:index")
}

func StepRSS(cfg *config.Config) Step {
	return StepFunc("rss", func(_ context.Context, sc *StepContext) error {
		site := registry.GetAs(sc.Registry, SiteK)
		pages := registry.GetAs(sc.Registry, PagesK)
		if cfg.RSS == nil {
			return nil
		}

		doc, err := transforms.RenderRSS(transforms.BuildRSS(pages, site, cfg.RSS))
		if err != nil {
			return err
		}
		sc.Manifest.Emit(manifest.TextArtefact(
			manifest.NewInternalClaim("rss", cfg.RSS.Output),
			doc,
		))
		return nil
	}, "pages:resolve")
}

func StepSitemap(cfg *config.Config) Step {
	return StepFunc("sitemap", func(_ context.Context, sc *StepContext) error {
		site := registry.GetAs(sc.Registry, SiteK)
		pages := registry.GetAs(sc.Registry, PagesK)
		if cfg.Sitemap == nil {
			return nil
		}

		doc, err := transforms.RenderSitemap(transforms.BuildSitemap(pages, site, cfg.Sitemap))
		if err != nil {
			return err
		}
		sc.Manifest.Emit(manifest.TextArtefact(
			manifest.NewInternalClaim("sitemap", cfg.Sitemap.Output),
			doc,
		))
		return nil
	}, "pages:resolve")
}
