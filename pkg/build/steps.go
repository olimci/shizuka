package build

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/git"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/structql"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

const (
	ConfigK       = manifest.K[*config.Config]("config")
	OptionsK      = manifest.K[*config.Options]("options")
	PagesK        = manifest.K[[]*transforms.Page]("pages")
	ContentFilesK = manifest.K[[]string]("contentfiles")
	SiteK         = manifest.K[*transforms.Site]("site")
	DBK           = manifest.K[*structql.DB]("db")
	TemplatesK    = manifest.K[*template.Template]("templates")
	BuildCtxK     = manifest.K[*BuildCtx]("buildctx")
	SiteGitK      = manifest.K[*transforms.SiteGitMeta]("sitegit")
)

func StepStatic() Step {
	return StepFunc("static", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		staticRoot, err := cfg.StaticSourcePath()
		if err != nil {
			return err
		}

		m := NewMinifier(cfg.Build.Minify)

		files, err := fileutil.WalkFiles(staticRoot)
		if err != nil {
			return err
		}

		for _, rel := range files.Values() {
			source, err := filepath.Rel(sc.SourceRoot, filepath.Join(staticRoot, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			source = filepath.ToSlash(source)
			sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceRoot, manifest.Claim{
				Owner:  "static",
				Source: source,
				Target: rel,
				Canon:  rel,
			}).Post(m))
		}

		return nil
	})
}

func StepContent(useGit bool) []Step {
	buildStep := StepFunc("pages:build", func(_ context.Context, sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, OptionsK)
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		tmpl := manifest.GetAs(sc.Manifest, TemplatesK)

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
	if useGit {
		resolveDeps = append(resolveDeps, "git")
	}

	resolve := StepFunc("pages:resolve", func(_ context.Context, sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, OptionsK)
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		buildCtx := manifest.GetAs(sc.Manifest, BuildCtxK)
		siteGit := manifest.GetAs(sc.Manifest, SiteGitK)
		if siteGit == nil {
			siteGit = &transforms.SiteGitMeta{}
		}

		site := &transforms.Site{
			Title:       cfg.Site.Title,
			Description: cfg.Site.Description,
			URL:         cfg.Site.URL,
			Params:      maps.Clone(cfg.Site.Params),
			Queries:     nil,
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
			slug, err := transforms.CleanSlug(slugSource)
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

		manifest.SetAs(sc.Manifest, SiteK, site)
		return nil
	}, resolveDeps...)

	templates := StepFunc("pages:templates", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		templateGlob, err := cfg.TemplateGlob()
		if err != nil {
			return err
		}

		funcs := transforms.DefaultTemplateFuncs()
		if db := manifest.GetAs(sc.Manifest, DBK); db != nil {
			maps.Copy(funcs, QueryFuncMap(db))
		}

		tmpl, err := parseTemplates(sc.SourceRoot, templateGlob, funcs)
		if err != nil {
			return fmt.Errorf("template glob %q: %w", templateGlob, err)
		}

		manifest.SetAs(sc.Manifest, TemplatesK, tmpl)
		return nil
	}, "pages:query")

	index := StepFunc("pages:index", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		contentRoot, err := cfg.ContentSourcePath()
		if err != nil {
			return err
		}

		files, err := fileutil.WalkFiles(contentRoot)
		if err != nil {
			return fmt.Errorf("content source %q: %w", contentRoot, err)
		}

		contentFiles := files.Values()
		slices.Sort(contentFiles)

		pages := make([]*transforms.Page, 0, len(contentFiles))
		pagesByDir := make(map[string][]*transforms.Page)
		dirIndexPage := make(map[string]*transforms.Page)

		for _, rel := range contentFiles {
			ext := path.Ext(rel)
			if !isPageSourceExt(ext) {
				continue
			}

			absSource := filepath.Join(contentRoot, filepath.FromSlash(rel))
			source, err := filepath.Rel(sc.SourceRoot, absSource)
			if err != nil {
				source = filepath.ToSlash(absSource)
			} else {
				source = filepath.ToSlash(source)
			}

			defaultURL := transforms.URLPathForContentPath(rel)
			page := &transforms.Page{
				SourcePath:  source,
				ContentPath: rel,
				URLPath:     defaultURL,
				OutputPath:  pathutil.OutputPathForURLPath(defaultURL),
				Assets:      make(map[string]*transforms.PageAsset),
			}

			builtPage, buildErr := transforms.BuildPage(sc.SourceRoot, source)
			if buildErr != nil {
				page.BuildError = buildErr
				pages = append(pages, page)
				sc.Error(buildErr, manifest.NewPageClaim(source, defaultURL))
				continue
			}
			page = builtPage

			params := maps.Clone(cfg.Content.Defaults.Params)
			maps.Copy(params, page.Params)
			page.Params = params
			page.SourcePath = source
			page.ContentPath = rel

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
			pageURLPath, err = transforms.CleanURLPath(pageURLPath)
			if err != nil {
				page.BuildError = fmt.Errorf("invalid url_path %q: %w", page.URLPath, err)
				sc.Error(page.BuildError, manifest.NewPageClaim(source, defaultURL))
				pages = append(pages, page)
				continue
			}
			page.URLPath = pageURLPath
			page.OutputPath = pathutil.OutputPathForURLPath(pageURLPath)

			aliases := make([]string, 0, len(page.Aliases))
			seenAliases := make(map[string]struct{}, len(page.Aliases))
			aliasErr := error(nil)
			for _, aliasRaw := range page.Aliases {
				alias, cleanErr := transforms.CleanURLPath(aliasRaw)
				if cleanErr != nil {
					aliasErr = fmt.Errorf("invalid alias %q: %w", aliasRaw, cleanErr)
					break
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
			if aliasErr != nil {
				page.BuildError = aliasErr
				sc.Error(page.BuildError, manifest.NewPageClaim(source, pageURLPath))
				pages = append(pages, page)
				continue
			}
			page.Aliases = aliases
			pages = append(pages, page)

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

		manifest.SetAs(sc.Manifest, PagesK, pages)
		manifest.SetAs(sc.Manifest, ContentFilesK, contentFiles)
		return nil
	})

	assets := StepFunc("pages:assets", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		if !cfg.Content.Bundles.Enabled {
			return nil
		}

		contentRoot, err := cfg.ContentSourcePath()
		if err != nil {
			return err
		}
		contentFiles := manifest.GetAs(sc.Manifest, ContentFilesK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		mode := cfg.Content.Bundles.Mode
		outputRoot := strings.TrimPrefix(path.Clean(cfg.Content.Bundles.Output), "/")
		outputRoot = strings.TrimPrefix(outputRoot, ".")
		outputRoot = strings.Trim(outputRoot, "/")
		if outputRoot == "" {
			outputRoot = "_assets"
		}

		owned := make(map[string]string)
		emitted := make(map[string]string)
		contentRootRel, err := filepath.Rel(sc.SourceRoot, contentRoot)
		if err != nil {
			return err
		}
		contentRootRel = filepath.ToSlash(filepath.Clean(contentRootRel))
		filesByDir, descendantsByDir := indexContentFiles(contentFiles)

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

			base := path.Base(rel)
			ext := path.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			dir := path.Dir(rel)
			if dir == "." {
				dir = ""
			}

			for _, child := range filesByDir[dir] {
				if child == rel {
					continue
				}

				childName := path.Base(child)
				childExt := path.Ext(childName)
				if isPageSourceExt(childExt) {
					continue
				}
				childStem := strings.TrimSuffix(childName, childExt)
				if childStem != stem {
					continue
				}

				source := path.Join(contentRootRel, child)
				if err := attachBundleAsset(sc, page, childName, source, owned, emitted, mode, outputRoot); err != nil {
					sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
				}
			}

			bundleDir := path.Join(dir, stem)
			if bundleDir != "" {
				if err := collectBundleDir(sc, page, bundleDir, contentRootRel, descendantsByDir[bundleDir], owned, emitted, mode, outputRoot); err != nil {
					sc.Error(err, manifest.NewPageClaim(page.SourcePath, page.URLPath))
				}
			}
		}

		return nil
	}, "pages:index")

	preprocess := StepFunc("pages:preprocess", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			if page.Source.Format != transforms.PageSourceFormatMarkdown {
				continue
			}

			body, links := preprocessMarkdown(page, pages, cfg.Content.Markdown.Wikilinks)
			page.Source.Preprocessed = body
			page.Links = links
		}

		return nil
	}, "pages:assets")

	render := StepFunc("pages:render", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
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
		site := manifest.GetAs(sc.Manifest, SiteK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		db, err := BuildQueryDB(site, pages)
		if err != nil {
			return err
		}

		manifest.SetAs(sc.Manifest, DBK, db)
		return nil
	}, "pages:resolve", "pages:preprocess")

	computed := StepFunc("pages:computed", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		tmpl := manifest.GetAs(sc.Manifest, TemplatesK)
		db := manifest.GetAs(sc.Manifest, DBK)

		siteQueries, err := ComputeSiteQueries(db, cfg.Site.Queries)
		if err != nil {
			return err
		}
		site.Queries = siteQueries

		expandedPages, err := ComputePageQueries(site, pages, tmpl, db)
		if err != nil {
			return err
		}
		manifest.SetAs(sc.Manifest, PagesK, expandedPages)
		return nil
	}, "pages:query", "pages:templates")

	steps := []Step{index, assets}
	if useGit {
		steps = append(steps, StepGit())
	}
	steps = append(steps, preprocess, resolve, query, templates, computed, render, buildStep)
	return steps
}

func StepGit() Step {
	return StepFunc("git", func(ctx context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		gitCfg := cfg.Content.Git
		if gitCfg == nil || !gitCfg.Enabled {
			return nil
		}

		root, err := filepath.Abs(sc.SourceRoot)
		if err != nil {
			return err
		}

		repo, err := git.Open(ctx, root)
		if err != nil || repo == nil {
			if errors.Is(err, git.ErrUnavailable) {
				return nil
			}
			return err
		}

		info, err := repo.SiteInfo(ctx)
		if err != nil {
			return err
		}
		manifest.SetAs(sc.Manifest, SiteGitK, info)

		for _, page := range pages {
			if page == nil || page.HasError() {
				continue
			}

			abs := filepath.Join(root, filepath.FromSlash(page.SourcePath))
			relPath, err := filepath.Rel(repo.Root(), abs)
			if err != nil {
				continue
			}
			relPath = filepath.ToSlash(filepath.Clean(relPath))
			if relPath == "." || relPath == "" || strings.HasPrefix(relPath, "../") {
				continue
			}

			info, err := repo.FileInfo(ctx, relPath, true)
			if err != nil {
				return err
			}

			page.Git = *info
			if gitCfg.Backfill {
				if page.Date.IsZero() {
					page.Date = info.Created
				}
				if page.Updated.IsZero() {
					page.Updated = info.Updated
				}
			}
			if !page.Updated.IsZero() {
				page.PubDate = page.Updated
			} else if !page.Date.IsZero() {
				page.PubDate = page.Date
			}
		}

		return nil
	}, "pages:index")
}

func StepHeaders() Step {
	return StepFunc("headers", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
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

func StepRedirects() Step {
	return StepFunc("redirects", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
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

func StepRSS() Step {
	return StepFunc("rss", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		if cfg.RSS == nil {
			return nil
		}

		sc.Manifest.Emit(manifest.TemplateArtefact(
			manifest.NewInternalClaim("rss", cfg.RSS.Output),
			transforms.RSSTemplate.Get(),
			transforms.BuildRSS(pages, site, cfg.RSS),
		))
		return nil
	}, "pages:resolve")
}

func StepSitemap() Step {
	return StepFunc("sitemap", func(_ context.Context, sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		if cfg.Sitemap == nil {
			return nil
		}

		sc.Manifest.Emit(manifest.TemplateArtefact(
			manifest.NewInternalClaim("sitemap", cfg.Sitemap.Output),
			transforms.SitemapTemplate.Get(),
			transforms.BuildSitemap(pages, site, cfg.Sitemap),
		))
		return nil
	}, "pages:resolve")
}
