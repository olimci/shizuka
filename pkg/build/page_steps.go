package build

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

func StepContent(cfg *config.Config, opts *options.Options) []Step {
	buildStep := StepFunc("pages:build", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		site := registry.GetAs(sc.Registry, SiteK)
		tmpl := registry.GetAs(sc.Registry, TemplatesK)

		m := NewMinifier(cfg.Build.Minify)

		for _, page := range pages {
			if page == nil {
				continue
			}

			claim := manifest.NewPageClaim(page.SourcePath, page.URLPath)
			if page.HasError() {
				if errTmpl := lookupErrPage(opts.PageErrTemplates, page.BuildError); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page:  *page,
						Site:  *site,
						Error: page.BuildError,
					}).Post(m))
				}
				continue
			}

			if page.Draft && !opts.Dev {
				continue
			}

			templateName := strings.TrimSpace(page.Template)
			if templateName == "" {
				err := errors.Join(ErrNoTemplate, errors.New("no template specified"))
				sc.Error(err, claim)
				if errTmpl := lookupErrPage(opts.PageErrTemplates, err); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page:  *page,
						Site:  *site,
						Error: err,
					}).Post(m))
				}
				continue
			}

			if tmpl.Lookup(templateName) == nil {
				err := errors.Join(ErrTemplateNotFound, fmt.Errorf("template %q not found", templateName))
				sc.Error(err, claim)
				if errTmpl := lookupErrPage(opts.PageErrTemplates, err); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page:  *page,
						Site:  *site,
						Error: err,
					}).Post(m))
				}
				continue
			}

			sc.Manifest.Emit(manifest.NamedTemplateArtefact(claim, templateName, tmpl, transforms.PageTemplate{
				Page: *page,
				Site: *site,
			}).Post(m))

			if page.Source.Format == transforms.PageSourceFormatMarkdown && cfg.Content.Raw.Markdown {
				target := "index.md"
				if page.URLPath != "" {
					target = path.Join(page.URLPath, "index.md")
				}
				sc.Manifest.Emit(manifest.TextArtefact(manifest.Claim{
					Owner:  "pages:raw",
					Source: page.SourcePath,
					Target: target,
					Canon:  pathutil.EnsureLeadingSlash(target),
				}, page.Source.RawDocument))
			}
		}

		return nil
	}, "pages:resolve", "pages:render", "pages:templates")

	resolveDeps := []string{"pages:index", "pages:assets", "pages:preprocess"}
	if cfg.Content.Git != nil {
		resolveDeps = append(resolveDeps, "git")
	}

	resolve := StepFunc("pages:resolve", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		buildCtx := registry.GetAs(sc.Registry, BuildCtxK)
		siteGit := registry.GetAs(sc.Registry, SiteGitK)
		if siteGit == nil {
			siteGit = &transforms.SiteGitMeta{}
		}

		site := &transforms.Site{
			Title:       cfg.Site.Title,
			Description: cfg.Site.Description,
			URL:         cfg.Site.URL,
			Params:      maps.Clone(cfg.Site.Params),
			Meta: transforms.SiteMeta{
				ConfigPath: opts.ConfigPath,
				IsDev:      opts.Dev,
				Git:        *siteGit,
				BuildTime:  buildCtx.StartTime,
			},
		}

		seenSlugs := make(map[string]string)
		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			slugSource := page.Slug
			if strings.TrimSpace(slugSource) == "" {
				slugSource = page.URLPath
			}
			slug, err := pathutil.CleanSlug(slugSource)
			if err != nil {
				sc.Error(fmt.Errorf("invalid slug %q: %w", slugSource, err), manifest.NewPageClaim(page.SourcePath, page.URLPath))
			} else {
				page.Slug = slug
				if slug != "" {
					if prev, ok := seenSlugs[slug]; ok {
						sc.Error(fmt.Errorf("duplicate slug %q (%s, %s)", slug, prev, page.SourcePath), manifest.NewPageClaim(page.SourcePath, page.URLPath))
					} else {
						seenSlugs[slug] = page.SourcePath
					}
				}
			}

			canon, err := pathutil.CanonicalPageURL(site.URL, page.URLPath)
			if err != nil {
				sc.Error(fmt.Errorf("canonical URL from site.url %q and page path %q: %w", site.URL, page.URLPath, err), manifest.NewPageClaim(page.SourcePath, page.URLPath))
			} else {
				page.Canon = canon
			}
		}

		registry.SetAs(sc.Registry, SiteK, site)
		return nil
	}, resolveDeps...)

	templates := StepFunc("pages:templates", func(_ context.Context, sc *StepContext) error {
		templateGlob, err := cfg.TemplateGlob()
		if err != nil {
			return err
		}

		funcs := transforms.DefaultTemplateFuncs()
		if db := registry.GetAs(sc.Registry, DBK); db != nil {
			maps.Copy(funcs, QueryFuncMap(db))
		}

		tmpl, err := parseTemplates(sc.SourceRoot, templateGlob, funcs)
		if err != nil {
			return fmt.Errorf("template glob %q: %w", templateGlob, err)
		}

		registry.SetAs(sc.Registry, TemplatesK, tmpl)
		return nil
	}, "pages:query")

	index := StepFunc("pages:index", func(_ context.Context, sc *StepContext) error {
		contentRoot, err := cfg.ContentSourcePath()
		if err != nil {
			return err
		}

		cache := registry.GetAs(sc.Cache, PageIndexCacheK)
		if cache == nil {
			cache = &pageIndexStepCache{Entries: make(map[string]pageIndexCacheEntry)}
			registry.SetAs(sc.Cache, PageIndexCacheK, cache)
		}
		if cache.Entries == nil {
			cache.Entries = make(map[string]pageIndexCacheEntry)
		}

		buildIndexedPage := func(rel string) (*transforms.Page, fileFingerprint, error) {
			absSource := filepath.Join(contentRoot, filepath.FromSlash(rel))
			source, err := filepath.Rel(sc.SourceRoot, absSource)
			if err != nil {
				source = filepath.ToSlash(absSource)
			} else {
				source = filepath.ToSlash(source)
			}

			fingerprint, err := statFingerprint(absSource)
			if err != nil {
				return nil, fileFingerprint{}, err
			}

			defaultURL := pathutil.URLPathForContentPath(rel)
			page := &transforms.Page{
				SourcePath:  source,
				ContentPath: rel,
				URLPath:     defaultURL,
				OutputPath:  pathutil.OutputPathForURLPath(defaultURL),
				Assets:      make(map[string]*transforms.PageAsset),
			}

			if !sc.ConfigChanged() {
				if entry, ok := cache.Entries[rel]; ok && entry.Fingerprint.Equal(fingerprint) && entry.Page != nil {
					return clonePageForIndexCache(entry.Page), fingerprint, nil
				}
			}

			builtPage, buildErr := transforms.BuildPage(sc.SourceRoot, source)
			if buildErr != nil {
				page.BuildError = buildErr
				sc.Error(buildErr, manifest.NewPageClaim(source, defaultURL))
				return page, fingerprint, nil
			}
			page = builtPage
			attachPageFileMeta(page, absSource)

			params := maps.Clone(cfg.Content.Defaults.Params)
			maps.Copy(params, page.Params)
			page.Params = params
			page.SourcePath = source
			page.ContentPath = rel
			page.Assets = make(map[string]*transforms.PageAsset)

			if page.Template == "" {
				page.Template = cfg.Content.Defaults.Template
			}
			if page.Section == "" {
				page.Section = cfg.Content.Defaults.Section
			}

			pageURLPath := strings.TrimSpace(page.URLPath)
			if pageURLPath == "" {
				pageURLPath = defaultURL
			}
			pageURLPath, err = pathutil.CleanURLPath(pageURLPath)
			if err != nil {
				page.BuildError = fmt.Errorf("invalid url_path %q: %w", page.URLPath, err)
				sc.Error(page.BuildError, manifest.NewPageClaim(source, defaultURL))
				return page, fingerprint, nil
			}
			page.URLPath = pageURLPath
			page.OutputPath = pathutil.OutputPathForURLPath(pageURLPath)

			aliases := make([]string, 0, len(page.Aliases))
			seenAliases := make(map[string]struct{}, len(page.Aliases))
			for _, aliasRaw := range page.Aliases {
				alias, cleanErr := pathutil.CleanURLPath(aliasRaw)
				if cleanErr != nil {
					page.BuildError = fmt.Errorf("invalid alias %q: %w", aliasRaw, cleanErr)
					sc.Error(page.BuildError, manifest.NewPageClaim(source, pageURLPath))
					return page, fingerprint, nil
				}
				if alias == page.URLPath {
					continue
				}
				if _, exists := seenAliases[alias]; exists {
					continue
				}
				seenAliases[alias] = struct{}{}
				aliases = append(aliases, alias)
			}
			page.Aliases = aliases
			return page, fingerprint, nil
		}

		var contentFiles []string
		if sc.ChangedPaths != nil && !sc.ConfigChanged() && !sc.MayHaveChangesUnder(contentRoot) {
			contentFiles = slices.Clone(cache.ContentFiles)
		} else {
			files, err := fileutil.WalkFiles(contentRoot)
			if err != nil {
				return fmt.Errorf("content source %q: %w", contentRoot, err)
			}
			contentFiles = files.Values()
			slices.Sort(contentFiles)
		}

		pages := make([]*transforms.Page, 0, len(contentFiles))
		pagesByDir := make(map[string][]*transforms.Page)
		dirIndexPage := make(map[string]*transforms.Page)
		nextEntries := make(map[string]pageIndexCacheEntry, len(cache.Entries))

		for _, rel := range contentFiles {
			ext := path.Ext(rel)
			if !isPageSourceExt(ext) {
				continue
			}

			page, fingerprint, err := buildIndexedPage(rel)
			if err != nil {
				return err
			}

			pages = append(pages, page)
			nextEntries[rel] = pageIndexCacheEntry{
				Fingerprint: fingerprint,
				Page:        clonePageForIndexCache(page),
			}

			dir := path.Dir(strings.TrimPrefix(rel, "/"))
			if dir == "." {
				dir = ""
			}
			pagesByDir[dir] = append(pagesByDir[dir], page)
			if strings.TrimSuffix(path.Base(rel), ext) == "index" {
				dirIndexPage[dir] = page
			}
		}

		for dir, items := range pagesByDir {
			for _, page := range items {
				if idx := dirIndexPage[dir]; idx != nil && idx != page {
					page.Parent = idx
					idx.Children = append(idx.Children, page)
					continue
				}

				parentDir := path.Dir(dir)
				if parentDir == "." {
					parentDir = ""
				}
				for {
					if parent := dirIndexPage[parentDir]; parent != nil && parent != page {
						page.Parent = parent
						parent.Children = append(parent.Children, page)
						break
					}
					if parentDir == "" {
						break
					}
					parentDir = path.Dir(parentDir)
					if parentDir == "." {
						parentDir = ""
					}
				}
			}
		}

		for _, page := range dirIndexPage {
			slices.SortStableFunc(page.Children, func(a, b *transforms.Page) int {
				return strings.Compare(a.URLPath, b.URLPath)
			})
		}

		registry.SetAs(sc.Registry, PagesK, pages)
		registry.SetAs(sc.Registry, ContentFilesK, contentFiles)
		cache.ContentFiles = slices.Clone(contentFiles)
		cache.Entries = nextEntries
		return nil
	})

	assets := StepFunc("pages:assets", func(_ context.Context, sc *StepContext) error {
		if !cfg.Content.Bundles.Enabled {
			return nil
		}

		contentRoot, err := cfg.ContentSourcePath()
		if err != nil {
			return err
		}
		contentFiles := registry.GetAs(sc.Registry, ContentFilesK)
		pages := registry.GetAs(sc.Registry, PagesK)
		mode := cfg.Content.Bundles.Mode
		outputRoot := strings.TrimPrefix(path.Clean(cfg.Content.Bundles.Output), "/")
		outputRoot = strings.TrimPrefix(outputRoot, ".")
		outputRoot = strings.Trim(outputRoot, "/")
		if outputRoot == "" {
			outputRoot = "_assets"
		}

		owned := make(map[string]string)
		emitted := make(map[string]string)
		cache := registry.GetAs(sc.Cache, PageAssetsCacheK)
		if cache == nil {
			cache = &pageAssetsStepCache{Entries: make(map[string]pageAssetsCacheEntry)}
			registry.SetAs(sc.Cache, PageAssetsCacheK, cache)
		}
		if cache.Entries == nil {
			cache.Entries = make(map[string]pageAssetsCacheEntry)
		}
		contentRootRel, err := filepath.Rel(sc.SourceRoot, contentRoot)
		if err != nil {
			return err
		}
		contentRootRel = filepath.ToSlash(filepath.Clean(contentRootRel))
		filesByDir, descendantsByDir := indexContentFiles(contentFiles)
		nextEntries := make(map[string]pageAssetsCacheEntry, len(cache.Entries))

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			page.Assets = make(map[string]*transforms.PageAsset)

			rel := page.ContentPath
			if rel == "" {
				sc.Error(fmt.Errorf("content path missing for %q", page.SourcePath), manifest.NewPageClaim(page.SourcePath, page.URLPath))
				continue
			}

			sources, err := pageBundleSources(page, filesByDir, descendantsByDir)
			if err != nil {
				sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
				continue
			}

			inputs, err := pageBundleInputs(contentRoot, sources)
			if err != nil {
				return err
			}

			cacheEntry, ok := cache.Entries[page.SourcePath]
			if ok && sameBundleInputs(cacheEntry.Inputs, inputs) {
				for _, asset := range clonePageAssets(cacheEntry.Assets, page) {
					if err := emitPageAsset(sc, page, asset, owned, emitted); err != nil {
						sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
					}
				}
				nextEntries[page.SourcePath] = pageAssetsCacheEntry{
					Inputs: maps.Clone(cacheEntry.Inputs),
					Assets: clonePageAssets(cacheEntry.Assets, nil),
				}
				continue
			}

			assets, err := buildPageAssets(sc, page, contentRootRel, sources, mode, outputRoot)
			if err != nil {
				sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
				continue
			}

			for _, asset := range assets {
				if err := emitPageAsset(sc, page, asset, owned, emitted); err != nil {
					sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
				}
			}
			nextEntries[page.SourcePath] = pageAssetsCacheEntry{
				Inputs: maps.Clone(inputs),
				Assets: clonePageAssets(assets, nil),
			}
		}

		cache.Entries = nextEntries
		return nil
	}, "pages:index")

	preprocess := StepFunc("pages:preprocess", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		lookup := newPageLookup(pages)

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			if page.Source.Format != transforms.PageSourceFormatMarkdown {
				continue
			}

			body, links := preprocessMarkdown(page, lookup, cfg.Content.Markdown.Wikilinks)
			page.Source.Preprocessed = body
			page.Links = links
		}

		return nil
	}, "pages:assets")

	render := StepFunc("pages:render", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)
		md := cfg.Content.Markdown.Goldmark.Build()

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			body := page.Source.Preprocessed
			if body == "" {
				body = page.Source.RawBody
			}

			switch page.Source.OutputKind {
			case transforms.PageOutputKindMarkdown:
				var buf strings.Builder
				if err := md.Convert([]byte(body), &buf); err != nil {
					return fmt.Errorf("render markdown %q: %w", page.SourcePath, err)
				}
				page.Body = template.HTML(buf.String())
			case transforms.PageOutputKindHTML:
				body = rewriteHTMLBundleLinks(body, page.Assets)
				page.Body = template.HTML(body)
			default:
				page.Body = template.HTML(body)
			}
		}

		return nil
	}, "pages:computed")

	query := StepFunc("pages:query", func(_ context.Context, sc *StepContext) error {
		pages := registry.GetAs(sc.Registry, PagesK)

		db, err := BuildQueryDB(pages)
		if err != nil {
			return err
		}

		registry.SetAs(sc.Registry, DBK, db)
		return nil
	}, "pages:resolve", "pages:preprocess")

	computed := StepFunc("pages:computed", func(_ context.Context, sc *StepContext) error {
		site := registry.GetAs(sc.Registry, SiteK)
		pages := registry.GetAs(sc.Registry, PagesK)
		tmpl := registry.GetAs(sc.Registry, TemplatesK)
		db := registry.GetAs(sc.Registry, DBK)

		expandedPages, err := ComputePageQueries(site, pages, tmpl, db)
		if err != nil {
			return err
		}
		registry.SetAs(sc.Registry, PagesK, expandedPages)
		return nil
	}, "pages:query", "pages:templates")

	steps := []Step{index, assets}
	if cfg.Content.Git != nil {
		steps = append(steps, StepGit(cfg))
	}
	steps = append(steps, preprocess, resolve, query, templates, computed, render, buildStep)
	return steps
}
