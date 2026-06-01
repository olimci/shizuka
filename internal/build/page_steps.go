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

	"github.com/olimci/shizuka/internal/config"
	"github.com/olimci/shizuka/internal/manifest"
	"github.com/olimci/shizuka/internal/markdown"
	"github.com/olimci/shizuka/internal/options"
	"github.com/olimci/shizuka/internal/registry"
	"github.com/olimci/shizuka/internal/transforms"
	"github.com/olimci/shizuka/internal/utils/pathutil"
	"github.com/olimci/shizuka/internal/utils/pool"
	"github.com/olimci/shizuka/internal/utils/tmplutil"
)

var (
	ErrNoTemplate       = errors.New("no template specified")
	ErrTemplateNotFound = errors.New("template not found")
)

func StepContent(cfg *config.Config, opts *options.Options) []Step {
	build := StepFunc("pages:build", func(_ context.Context, sc *StepContext) error {
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

		var built, errored, drafts int
		for _, page := range pages {
			claim := manifest.NewPageClaim(page.SourcePath, page.Path)

			if page.Error != nil {
				errored++
				if err := emitDebug(page, claim, page.Error); err != nil {
					return err
				}
				continue
			}

			if page.Draft && !opts.Dev {
				drafts++
				continue
			}

			if page.Template == "" {
				errored++
				sc.Error(ErrNoTemplate, claim)
				if err := emitDebug(page, claim, ErrNoTemplate); err != nil {
					return err
				}
				continue
			}

			if tmpl.Lookup(page.Template) == nil {
				errored++
				err := fmt.Errorf("%w: %q", ErrTemplateNotFound, page.Template)
				sc.Error(err, claim)
				if err := emitDebug(page, claim, err); err != nil {
					return err
				}
				continue
			}

			built++
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

		sc.Logger.Info("pages built", "built", built, "errored", errored, "drafts_skipped", drafts)
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
		pages := registry.Get(sc.Registry, PagesK)
		funcs := tmplutil.DefaultFuncs()
		md := markdown.Build(cfg.Content.Markdown, markdownOptions(cfg.Content.Markdown, pages, opts.Dev))
		funcs["markdown"] = func(value any) (template.HTML, error) {
			var buf strings.Builder
			if err := md.Convert(fmt.Append(nil, value), &buf); err != nil {
				return "", err
			}
			return template.HTML(buf.String()), nil
		}
		maps.Copy(funcs, QueryFuncMap(registry.Get(sc.Registry, DBK)))
		maps.Copy(funcs, paginationFuncMap())

		templateGlob := path.Join(cfg.Paths.Templates, "html", "**", "*.tmpl")
		tmpl, err := parseRequiredTemplates(sc.Source.FS(), templateGlob, funcs)
		if err != nil {
			return err
		}

		registry.Set(sc.Registry, TemplatesK, tmpl)
		var tmplCount int
		if tmpl != nil {
			tmplCount = len(tmpl.Templates())
		}
		sc.Logger.Info("templates parsed", "count", tmplCount, "glob", templateGlob)
		return nil
	}, "pages:query").Registry(registry.R(PagesK), registry.R(DBK), registry.W(TemplatesK))

	index := StepFunc("pages:index", func(_ context.Context, sc *StepContext) error {
		contentRoot := cfg.Paths.Content
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

		batch := pool.NewBatch[pageResult](sc.Pool)
		for i, rel := range pageSources {
			batch.Go(func(_ context.Context) (pageResult, error) {
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
			})
		}

		results, err := batch.Wait()
		if err != nil {
			return err
		}
		pages := make([]*transforms.Page, len(pageSources))
		for _, result := range results {
			pages[result.Index] = result.Page
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
		sc.Logger.Info("pages indexed", "discovered", len(pageSources), "kept", len(pages))
		return nil
	}).Registry(registry.W(PagesK))

	render := StepFunc("pages:render", func(_ context.Context, sc *StepContext) error {
		pages := registry.Get(sc.Registry, PagesK)
		publicMD := markdown.Build(cfg.Content.Markdown, markdownOptions(cfg.Content.Markdown, pages, false))
		draftMD := markdown.Build(cfg.Content.Markdown, markdownOptions(cfg.Content.Markdown, pages, true))
		var mdTemplates *template.Template
		if cfg.Content.Markdown.Components {
			templateGlob := path.Join(cfg.Paths.Templates, "md", "**", "*.tmpl")
			tmpl, err := parseOptionalTemplates(sc.Source.FS(), templateGlob, tmplutil.DefaultFuncs())
			if err != nil {
				return err
			}
			mdTemplates = tmpl
		}

		batch := pool.NewBatch[*transforms.Page](sc.Pool)
		preprocessed := 0
		for _, page := range pages {
			if page.Error != nil || page.Preprocess == "" {
				continue
			}
			preprocessed++

			batch.Go(func(_ context.Context) (*transforms.Page, error) {
				switch page.Preprocess {
				case "markdown":
					rawBody := page.RawBody
					if mdTemplates != nil {
						rendered, err := renderMarkdownComponentTemplate(mdTemplates, page)
						if err != nil {
							return nil, err
						}
						rawBody = rendered
					}

					md := publicMD
					if page.Draft {
						md = draftMD
					}
					doc, err := markdown.Render(md, page.SourcePath, rawBody)
					if err != nil {
						return nil, err
					}
					page.Body = doc.Body
					page.Sections = doc.Sections
					page.ToC = doc.ToC

					page.Preprocess = ""
				default:
					return nil, fmt.Errorf("unknown page preprocessor %q for %q", page.Preprocess, page.SourcePath)
				}
				return page, nil
			})
		}

		_, err := batch.Wait()
		if err == nil {
			sc.Logger.Info("pages preprocessed", "count", preprocessed)
		}
		return err
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
		sc.Logger.Info("data loaded", "tables", len(dataTables), "pages", len(pages))
		return nil
	}, "pages:render").Registry(registry.R(PagesK), registry.W(DBK))

	return []Step{index, resolve, render, query, templates, build}
}

func markdownOptions(cfg config.ConfigContentMarkdown, pages []*transforms.Page, includeDrafts bool) markdown.Options {
	if !cfg.Wikilinks {
		return markdown.Options{}
	}

	routes := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		if page.Error != nil || page.Draft && !includeDrafts {
			continue
		}
		routes[page.Path] = struct{}{}
	}

	return markdown.Options{
		TargetResolver: func(target string) (string, error) {
			route, err := wikilinkRouteTarget(target)
			if err != nil {
				return "", err
			}
			if _, ok := routes[route]; !ok {
				return "", fmt.Errorf("missing route %q", route)
			}
			return route, nil
		},
	}
}

func wikilinkRouteTarget(target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("target is empty")
	}
	if strings.HasPrefix(target, "/") {
		return pathutil.ValidateRoutePath(target)
	}
	route := "/" + target
	if !strings.HasSuffix(route, "/") {
		route += "/"
	}
	return pathutil.ValidateRoutePath(route)
}
