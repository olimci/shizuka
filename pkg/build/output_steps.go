package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
	"github.com/olimci/shizuka/pkg/utils/gitutil"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

func StepStatic(cfg *config.Config) Step {
	return StepFunc("static", func(_ context.Context, sc *StepContext) error {
		staticRoot := cfg.StaticSourcePath()
		info, err := fs.Stat(sc.Source.FS(), staticRoot)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("static source %q: %w", staticRoot, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("static source %q is not a directory", staticRoot)
		}

		m := NewMinifier(cfg.Build.Minifier)
		err = fs.WalkDir(sc.Source.FS(), staticRoot, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := pathutil.RelPathWithin(staticRoot, filePath)
			if err != nil {
				return err
			}
			source := pathutil.JoinSlashRel(staticRoot, rel)
			claim := manifest.Claim{
				Owner:  "static",
				Source: source,
				Target: rel,
				Canon:  rel,
			}
			return sc.Manifest.Emit(manifest.StaticArtefact(sc.Source.FS(), claim).Post(m))
		})
		if err != nil {
			return fmt.Errorf("static source %q: %w", staticRoot, err)
		}
		return nil
	})
}

func StepGit(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("git", func(ctx context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		gitCfg := cfg.Content.Git
		if gitCfg == nil {
			return nil
		}

		root, err := filepath.Abs(sc.Source.Name())
		if err != nil {
			return err
		}

		cache := (*gitStepCache)(nil)
		if sc.Cache != nil {
			cache = registry.Get(sc.Cache, GitCacheK)
			if cache == nil {
				cache = &gitStepCache{}
				registry.Set(sc.Cache, GitCacheK, cache)
			}
		} else {
			cache = &gitStepCache{}
		}
		if cache.Files == nil {
			cache.Files = make(map[string]gitFileCacheEntry)
		}

		repo := cache.Repo
		if repo == nil && !cache.Unavailable {
			repo, err = gitutil.Open(ctx, root)
			if err != nil || repo == nil {
				if errors.Is(err, gitutil.ErrUnavailable) {
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
			info, err = repo.Repo(ctx)
			if err != nil {
				return err
			}
			cache.Site = info
			cache.SiteExpires = now.Add(gitTTL)
		}
		registry.Set(sc.Registry, SiteGitK, info)

		activeFiles := make(map[string]struct{}, len(pages))
		for _, page := range pages {
			if page.Error != nil {
				continue
			}

			abs := filepath.Join(root, filepath.FromSlash(page.SourcePath))
			relPath, err := filepath.Rel(root, abs)
			if err != nil {
				continue
			}
			relPath = filepath.ToSlash(filepath.Clean(relPath))
			if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
				continue
			}
			activeFiles[relPath] = struct{}{}

			fingerprint, err := fileutil.Info(abs)
			if err != nil {
				return err
			}

			pageInfo := (*transforms.PageGitMeta)(nil)
			if entry, ok := cache.Files[relPath]; ok && entry.Fingerprint.Equal(fingerprint) && now.Before(entry.ExpiresAt) && entry.Info != nil {
				pageInfo = entry.Info
			} else {
				pageInfo, err = repo.File(ctx, relPath, true)
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
	}, "pages:index").Registry(registry.W(PagesK), registry.W(SiteGitK)).Cache(registry.W(GitCacheK))).AddDependency("pages:resolve", "git")
}

func applyGitMetadata(page *transforms.Page, info *transforms.PageGitMeta, backfill bool) {
	if info == nil {
		return
	}

	page.Git = *info
	if backfill {
		if page.Created.IsZero() && !info.Created.IsZero() {
			page.Created = info.Created
		}
		if page.Updated.IsZero() && !info.Updated.IsZero() {
			page.Updated = info.Updated
		}
	}
	if !page.Updated.IsZero() {
		page.PubDate = page.Updated
	} else if !page.Created.IsZero() {
		page.PubDate = page.Created
	}
}

func StepHeaders(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("headers", func(_ context.Context, sc *StepContext) error {
		headers := make(map[string]map[string]string, len(cfg.Artefacts.Headers.Values))
		for path, kvs := range cfg.Artefacts.Headers.Values {
			headers[path] = maps.Clone(kvs)
		}
		if len(headers) == 0 {
			return nil
		}

		return sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("headers", cfg.Artefacts.Headers.Path),
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
	}))
}

func StepRedirects(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("redirects", func(_ context.Context, sc *StepContext) error {
		redirects := slices.Clone(cfg.Artefacts.Redirects.Entries)
		if len(redirects) == 0 {
			return nil
		}

		redirectRank := func(redirect config.Redirect) int {
			if strings.Contains(redirect.From, "*") {
				return 2
			}
			return 1
		}
		slices.SortStableFunc(redirects, func(a, b config.Redirect) int {
			aRank := redirectRank(a)
			bRank := redirectRank(b)
			if aRank != bRank {
				return aRank - bRank
			}
			return 0
		})

		return sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("redirects", cfg.Artefacts.Redirects.Path),
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
	}))
}

func StepRSS(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("rss", func(_ context.Context, sc *StepContext) error {
		site := registry.Get(sc.Registry, SiteK)
		pages := registry.Get(sc.Registry, PagesK)

		doc, err := transforms.RenderRSS(transforms.BuildRSS(pages, site, cfg.Artefacts.RSS))
		if err != nil {
			return err
		}
		return sc.Manifest.Emit(manifest.TextArtefact(
			manifest.NewInternalClaim("rss", cfg.Artefacts.RSS.Path),
			doc,
		))
	}, "pages:resolve").Registry(registry.R(SiteK), registry.R(PagesK)))
}

func StepSitemap(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("sitemap", func(_ context.Context, sc *StepContext) error {
		site := registry.Get(sc.Registry, SiteK)
		pages := registry.Get(sc.Registry, PagesK)

		doc, err := transforms.RenderSitemap(transforms.BuildSitemap(pages, site, cfg.Artefacts.Sitemap))
		if err != nil {
			return err
		}
		return sc.Manifest.Emit(manifest.TextArtefact(
			manifest.NewInternalClaim("sitemap", cfg.Artefacts.Sitemap.Path),
			doc,
		))
	}, "pages:resolve").Registry(registry.R(SiteK), registry.R(PagesK)))
}

func StepRobots(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("robots", func(_ context.Context, sc *StepContext) error {
		site := registry.Get(sc.Registry, SiteK)
		pages := registry.Get(sc.Registry, PagesK)

		doc := transforms.RenderRobots(transforms.BuildRobots(pages, site, cfg.Artefacts.Robots, cfg.Artefacts.Sitemap))
		return sc.Manifest.Emit(manifest.TextArtefact(
			manifest.NewInternalClaim("robots", cfg.Artefacts.Robots.Path),
			doc,
		))
	}, "pages:resolve").Registry(registry.R(SiteK), registry.R(PagesK)))
}

func StepNotFound(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("not_found", func(_ context.Context, sc *StepContext) error {
		site := registry.Get(sc.Registry, SiteK)
		tmpl := registry.Get(sc.Registry, TemplatesK)

		claim := manifest.NewInternalClaim("not_found", cfg.Artefacts.NotFound.Path)
		templateName := cfg.Artefacts.NotFound.Template
		if templateName == "" {
			templateName = "404"
		}

		if pageTemplate := tmpl.Lookup(templateName); pageTemplate != nil {
			return sc.Pool.Go(func(_ context.Context) error {
				return emitRenderedTemplate(sc, claim, tmpl, templateName, transforms.PageTemplate{
					Site: site.Tmpl(),
				}, NewMinifier(cfg.Build.Minifier))
			})
		}
		if cfg.Artefacts.NotFound.Template != "" {
			return fmt.Errorf("not_found template %q not found", cfg.Artefacts.NotFound.Template)
		}

		return sc.Manifest.Emit(manifest.TextArtefact(claim, "404 Not Found\n"))
	}, "pages:templates").Registry(registry.R(SiteK), registry.R(TemplatesK)))
}

func StepMeta(cfg *config.Config) StepPatch {
	return StepPatchFunc(StepFunc("meta", func(_ context.Context, sc *StepContext) error {
		site := registry.Get(sc.Registry, SiteK)
		data := struct {
			Site   *transforms.Site `json:"site"`
			Config *config.Config   `json:"config"`
		}{
			Site:   site,
			Config: cfg,
		}

		return sc.Pool.Go(func(_ context.Context) error {
			claim := manifest.NewInternalClaim("meta", cfg.Artefacts.Meta.Path)
			if cfg.Artefacts.Meta.JSON {
				jsonClaim := manifest.NewInternalClaim("meta", "_shizuka.json")
				body, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					sc.Error(err, jsonClaim)
					return nil
				}
				if err := sc.Manifest.Emit(manifest.TextArtefact(jsonClaim, string(body)+"\n")); err != nil {
					return err
				}
			}
			return emitDebugTemplate(sc, claim, data, NewMinifier(cfg.Build.Minifier))
		})
	}, "pages:resolve").Registry(registry.R(SiteK)))
}
