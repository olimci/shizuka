package build

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"strings"

	"github.com/olimci/shizuka/pkg/build/embed"
	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
	"github.com/olimci/shizuka/pkg/registry"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/shizuka/pkg/utils/pool"
	"github.com/olimci/shizuka/pkg/utils/tmplutil"
)

var (
	ErrNoTemplate       = errors.New("no template specified")
	ErrTemplateNotFound = errors.New("template not found")
)

func StepContent(cfg *config.Config, opts *options.Options) []Step {
	buildStep := StepFunc("pages:build", func(_ context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		site := registry.Get(sc.Registry, SiteK)
		tmpl := registry.Get(sc.Registry, TemplatesK)
		minifier := NewMinifier(cfg.Build.Minifier)

		emitDebug := func(page *transforms.Page, claim manifest.Claim, err error) error {
			if !opts.Dev {
				return nil
			}
			return sc.Pool.Go(func(_ context.Context) error {
				return emitDebugTemplate(sc, claim, transforms.PageTemplate{
					Error: err,
					Page:  page.Tmpl(),
					Site:  site.Tmpl(),
				}, minifier)
			})
		}

		for _, page := range pages {
			claim := manifest.NewPageClaim(page.SourcePath, page.Path)

			if page.Error != nil {
				if err := emitDebug(page, claim, page.Error); err != nil {
					return err
				}
				continue
			}

			if page.Draft && !opts.Dev {
				continue
			}

			if page.Template == "" {
				sc.Error(ErrNoTemplate, claim)
				if err := emitDebug(page, claim, ErrNoTemplate); err != nil {
					return err
				}
				continue
			}

			if tmpl.Lookup(page.Template) == nil {
				err := fmt.Errorf("%w: %q", ErrTemplateNotFound, page.Template)
				sc.Error(err, claim)
				if err := emitDebug(page, claim, err); err != nil {
					return err
				}
				continue
			}

			if err := sc.Pool.Go(func(_ context.Context) error {
				return renderPageTemplate(sc, pageRenderRequest{
					Claim:        claim,
					TemplateName: page.Template,
					Templates:    tmpl,
					Page:         page.Tmpl(),
					Site:         site.Tmpl(),
					Minifier:     minifier,
				})
			}); err != nil {
				return err
			}
		}

		return nil
	}, "pages:templates").Registry(registry.R(PagesK), registry.R(SiteK), registry.R(TemplatesK))

	resolve := StepFunc("pages:resolve", func(_ context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		buildCtx := registry.Get(sc.Registry, BuildCtxK)
		siteGit, _ := registry.GetOk(sc.Registry, SiteGitK)
		if siteGit == nil {
			siteGit = &transforms.SiteGitMeta{}
		}

		site := &transforms.Site{
			Title:       cfg.Site.Title,
			Description: cfg.Site.Description,
			URL:         cfg.Site.URL,
			Params:      maps.Clone(cfg.Site.Params),
			Dev:         opts.Dev,
			Git:         *siteGit,
			BuildTime:   buildCtx.StartTime,
		}

		for _, page := range pages {
			if page.Error != nil {
				continue
			}

			canon, err := pathutil.CanonicalPageURL(site.URL, page.Path)
			if err != nil {
				sc.Error(fmt.Errorf("canonical URL from site.url %q and page path %q: %w", site.URL, page.Path, err), manifest.NewPageClaim(page.SourcePath, page.Path))
				continue
			}
			page.Canon = canon
		}

		registry.Set(sc.Registry, SiteK, site)
		return nil
	}, "pages:index").Registry(registry.W(PagesK), registry.R(BuildCtxK), registry.RX(SiteGitK), registry.W(SiteK))

	templates := StepFunc("pages:templates", func(_ context.Context, sc *StepContext) error {
		funcs := tmplutil.DefaultFuncs()
		md := cfg.Content.Markdown.Goldmark.Build()
		funcs["markdown"] = func(value any) (template.HTML, error) {
			var buf strings.Builder
			if err := md.Convert(fmt.Append(nil, value), &buf); err != nil {
				return "", err
			}
			return template.HTML(buf.String()), nil
		}
		maps.Copy(funcs, QueryFuncMap(registry.Get(sc.Registry, DBK)))
		maps.Copy(funcs, paginationFuncMap())

		templateGlob := cfg.TemplateGlob()
		tmpl, err := parseTemplates(sc.Source.FS(), templateGlob, funcs)
		if err != nil {
			return fmt.Errorf("template glob %q: %w", templateGlob, err)
		}

		registry.Set(sc.Registry, TemplatesK, tmpl)
		return nil
	}, "pages:query").Registry(registry.R(DBK), registry.W(TemplatesK))

	index := StepFunc("pages:index", func(_ context.Context, sc *StepContext) error {
		contentRoot := cfg.ContentSourcePath()
		var pageSources []string

		if err := fs.WalkDir(sc.Source.FS(), contentRoot, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			rel, err := pathutil.RelPathWithin(contentRoot, filePath)
			if err != nil {
				return err
			}
			if !isPageSourceExt(path.Ext(rel)) {
				return nil
			}

			pageSources = append(pageSources, rel)
			return nil
		}); err != nil {
			return fmt.Errorf("content source %q: %w", contentRoot, err)
		}

		type pageResult struct {
			Index int
			Page  *transforms.Page
		}

		stream := pool.NewStream[pageResult](sc.Pool, len(pageSources))
		for i, rel := range pageSources {
			if err := stream.Go(func(_ context.Context) (pageResult, error) {
				source := pathutil.JoinSlashRel(contentRoot, rel)
				routePath, err := pathutil.RoutePathForContentPath(rel)
				if err != nil {
					sc.Error(err, manifest.NewPageClaim(source, ""))
					return pageResult{Index: i}, nil
				}

				page, err := transforms.BuildPage(
					sc.Source.FS(),
					source,
					cfg.Content.Defaults.Section,
					cfg.Content.Defaults.Global,
					cfg.Content.Defaults.Sections,
				)
				if err != nil {
					page = &transforms.Page{
						SourcePath:  source,
						ContentPath: rel,
						Path:        routePath,
						OutputPath:  pathutil.OutputPathForRoutePath(routePath),
						Error:       err,
					}
					sc.Error(err, manifest.NewPageClaim(source, routePath))
					return pageResult{Index: i, Page: page}, nil
				}

				page.SourcePath = source
				page.ContentPath = rel
				page.Path = routePath
				page.OutputPath = pathutil.OutputPathForRoutePath(routePath)
				attachPageFileMeta(page, filepath.Join(sc.Source.Name(), filepath.FromSlash(source)))
				return pageResult{Index: i, Page: page}, nil
			}); err != nil {
				return err
			}
		}

		pages := make([]*transforms.Page, len(pageSources))
		for result := range stream.Results() {
			pages[result.Index] = result.Page
		}
		if err := stream.Await(); err != nil {
			return err
		}

		seenPaths := make(map[string]string, len(pages))
		usedSlugs := make(map[string]string, len(pages))
		write := 0
		for _, page := range pages {
			if page == nil {
				continue
			}

			if page.Error == nil {
				routePath, err := pathutil.ValidateRoutePath(page.Path)
				claim := manifest.NewPageClaim(page.SourcePath, page.Path)
				if err != nil {
					page.Error = fmt.Errorf("invalid route path %q: %w", page.Path, err)
					sc.Error(page.Error, claim)
				} else if previous, exists := seenPaths[routePath]; exists {
					page.Error = fmt.Errorf("duplicate route path %q (%s, %s)", routePath, previous, page.SourcePath)
					sc.Error(page.Error, manifest.NewPageClaim(page.SourcePath, routePath))
				} else {
					page.Path = routePath
					seenPaths[routePath] = page.SourcePath
				}
			}

			if page.Error == nil {
				claim := manifest.NewPageClaim(page.SourcePath, page.Path)
				if page.Slug == "" {
					slug, err := generatedSlug(page.SourcePath, page.Path, usedSlugs)
					if err != nil {
						return err
					}
					page.Slug = slug
					usedSlugs[slug] = page.SourcePath
				} else if !slugPattern.MatchString(page.Slug) {
					page.Error = fmt.Errorf("invalid slug %q: slug must contain only A-Z, a-z, 0-9, _, or -", page.Slug)
					sc.Error(page.Error, claim)
				} else if previous, exists := usedSlugs[page.Slug]; exists {
					sc.Error(fmt.Errorf("duplicate slug %q (%s, %s)", page.Slug, previous, page.SourcePath), claim)
					slug, err := generatedSlug(page.SourcePath, page.Path, usedSlugs)
					if err != nil {
						return err
					}
					page.Slug = slug
					usedSlugs[slug] = page.SourcePath
				} else {
					usedSlugs[page.Slug] = page.SourcePath
				}
			}

			pages[write] = page
			write++
		}
		pages = pages[:write]

		registry.Set(sc.Registry, PagesK, pages)
		return nil
	}).Registry(registry.W(PagesK))

	render := StepFunc("pages:render", func(_ context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		md := cfg.Content.Markdown.Goldmark.Build()

		stream := pool.NewStream[*transforms.Page](sc.Pool, len(pages))
		for _, page := range pages {
			if page.Error != nil || page.Preprocess == "" {
				continue
			}

			if err := stream.Go(func(_ context.Context) (*transforms.Page, error) {
				switch page.Preprocess {
				case "markdown":
					var buf strings.Builder
					if err := md.Convert([]byte(page.RawBody), &buf); err != nil {
						return nil, fmt.Errorf("render markdown %q: %w", page.SourcePath, err)
					}
					page.Body = template.HTML(buf.String())
					page.Preprocess = ""
				default:
					return nil, fmt.Errorf("unknown page preprocessor %q for %q", page.Preprocess, page.SourcePath)
				}
				return page, nil
			}); err != nil {
				return err
			}
		}

		for range stream.Results() { // TODO: could we make something like stream.Consume() for this?
		}
		return stream.Await()
	}, "pages:resolve").Registry(registry.W(PagesK))

	query := StepFunc("pages:query", func(_ context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		dataTables, err := loadDataTables(sc.Source.FS(), cfg.Paths.Data)
		if err != nil {
			return err
		}

		db, err := buildDB(pages, dataTables)
		if err != nil {
			return err
		}

		registry.Set(sc.Registry, DBK, db)
		return nil
	}, "pages:render").Registry(registry.R(PagesK), registry.W(DBK))

	return []Step{index, resolve, render, query, templates, buildStep}
}

func emitDebugTemplate(sc *StepContext, claim manifest.Claim, data any, pp manifest.PostProcessor) error {
	var buf strings.Builder
	err := embed.Templates.Get().ExecuteTemplate(&buf, "debug", data)
	if tmplutil.IsDiscard(err) {
		return nil
	}
	if err != nil {
		sc.Error(err, claim)
		return nil
	}
	return sc.Manifest.Emit(manifest.TextArtefact(claim, buf.String()).Post(pp))
}

func emitRenderedTemplate(sc *StepContext, claim manifest.Claim, tmpl *template.Template, name string, data any, pp manifest.PostProcessor) error {
	var buf strings.Builder
	err := tmpl.ExecuteTemplate(&buf, name, data)
	if tmplutil.IsDiscard(err) {
		return nil
	}
	if err != nil {
		sc.Error(err, claim)
		return nil
	}
	return sc.Manifest.Emit(manifest.TextArtefact(claim, buf.String()).Post(pp))
}
