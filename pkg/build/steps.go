package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

const (
	ConfigK    = manifest.K[*config.Config]("config")
	OptionsK   = manifest.K[*config.Options]("options")
	PagesK     = manifest.K[[]*transforms.Page]("pages")
	SiteK      = manifest.K[*transforms.Site]("site")
	TemplatesK = manifest.K[*template.Template]("templates")
	BuildCtxK  = manifest.K[*BuildCtx]("buildctx")
)

// StepStatic attatches static files
func StepStatic() Step {
	return StepFunc("static", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)

		m := NewMinifier(cfg.Build.Minify)

		sourceRoot := cfg.Build.Steps.Static.Source
		targetRoot := cfg.Build.Steps.Static.Destination

		files, err := fileutils.WalkFiles(sourceRoot)
		if err != nil {
			return err
		}

		for _, rel := range files.Values() {
			sc.Manifest.Emit(manifest.StaticArtefact(manifest.Claim{
				Owner:  "static",
				Source: filepath.Join(sourceRoot, rel),
				Target: filepath.Join(targetRoot, rel),
				Canon:  filepath.Join(targetRoot, rel),
			}).Post(m))
		}

		return nil
	})
}

// StepContent builds pages
func StepContent() []Step {
	// build creates the manifest artefacts for the pages
	build := StepFunc("pages:build", func(sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, OptionsK)
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		tmpl := manifest.GetAs(sc.Manifest, TemplatesK)

		m := NewMinifier(cfg.Build.Minify)

		for _, page := range pages {
			claim := page.Meta.Claim.Own("pages:build")

			if page.Meta.Err != nil {
				if errTmpl := lookupErrPage(opts.PageErrTemplates, page.Meta.Err); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page: *page,
						Site: *site,
					}).Post(m))
				}

				continue
			}

			if tmpl.Lookup(page.Meta.Template) == nil {
				if page.Meta.Template == "" {
					page.Meta.Err = errors.Join(ErrNoTemplate, errors.New("no template specified"))
					sc.Errorf(page.Meta.Err, "page %s: no template specified", claim)
				} else {
					page.Meta.Err = errors.Join(ErrTemplateNotFound, fmt.Errorf("template %q not found", page.Meta.Template))
					sc.Errorf(page.Meta.Err, "template %q not found", page.Meta.Template)
				}

				if errTmpl := lookupErrPage(opts.PageErrTemplates, page.Meta.Err); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page: *page,
						Site: *site,
					}).Post(m))
				}

				continue
			}

			sc.Manifest.Emit(manifest.NamedTemplateArtefact(claim, page.Meta.Template, tmpl, transforms.PageTemplate{
				Page: *page,
				Site: *site,
			}).Post(m))
		}

		return nil
	}, "pages:resolve", "pages:templates")

	// resolve creates the manifest registry entries for site information.
	resolve := StepFunc("pages:resolve", func(sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, OptionsK)
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		buildCtx := manifest.GetAs(sc.Manifest, BuildCtxK)

		site := &transforms.Site{
			Title:       cfg.Site.Title,
			Description: cfg.Site.Description,
			URL:         cfg.Site.URL,

			Meta: transforms.SiteMeta{
				ConfigPath: opts.ConfigPath,
				IsDev:      opts.Dev,

				BuildTime:       buildCtx.StartTime,
				BuildTimeString: buildCtx.StartTimestring,
			},
		}

		for _, page := range pages {
			if page.Meta.Err != nil {
				continue
			}

			if page.Featured {
				site.Collections.Featured = append(site.Collections.Featured, page.Lite())
			}

			if page.Draft {
				site.Collections.Drafts = append(site.Collections.Drafts, page.Lite())
			}

			site.Collections.All = append(site.Collections.All, page.Lite())
		}

		site.Collections.Latest = slices.Clone(site.Collections.All)
		slices.SortFunc(site.Collections.Latest, func(a, b *transforms.PageLite) int {
			if a.Date.After(b.Date) {
				return -1
			} else if a.Date.Before(b.Date) {
				return +1
			}
			return 0
		})

		site.Collections.RecentlyUpdated = slices.Clone(site.Collections.All)
		slices.SortFunc(site.Collections.RecentlyUpdated, func(a, b *transforms.PageLite) int {
			if a.Date.After(b.Date) {
				return -1
			} else if a.Date.Before(b.Date) {
				return +1
			}
			return 0
		})

		manifest.SetAs(sc.Manifest, SiteK, site)

		return nil
	}, "pages:index")

	templates := StepFunc("pages:templates", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)

		tmpl, err := parseTemplates(config.Build.Steps.Content.TemplateGlob, transforms.DefaultTemplateFuncs())
		if err != nil {
			return fmt.Errorf("failed to parse templates: %w", err)
		}

		manifest.SetAs(sc.Manifest, TemplatesK, tmpl)

		return nil
	})

	// index indexes pages and creates the manifest registry entries for page information.
	index := StepFunc("pages:index", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)

		sourceRoot := cfg.Build.Steps.Content.Source
		targetRoot := cfg.Build.Steps.Content.Destination

		md := cfg.Build.Steps.Content.GoldmarkConfig.Build()

		files, err := fileutils.WalkFiles(sourceRoot)
		if err != nil {
			return err
		}

		pages := make([]*transforms.Page, 0, len(files.Values()))

		for _, rel := range files.Values() {
			claim := manifest.NewPageClaim(sourceRoot, targetRoot, rel).Own("build:index")

			if filepath.Ext(rel) == ".html" {
				sc.Manifest.Emit(manifest.StaticArtefact(claim))
				continue
			}

			page, err := transforms.BuildPage(claim, md)
			if err != nil {
				pages = append(pages, &transforms.Page{
					Meta: transforms.PageMeta{
						Claim: claim,
						Err:   err,
					},
				})

				sc.Errorf(err, "failed to build page %s", claim)

				continue
			}

			params := maps.Clone(cfg.Build.Steps.Content.DefaultParams)
			liteParams := maps.Clone(cfg.Build.Steps.Content.DefaultLiteParams)

			maps.Copy(params, page.Params)
			maps.Copy(liteParams, page.LiteParams)

			page.Params = params
			page.LiteParams = liteParams

			pages = append(pages, page)
		}

		manifest.SetAs(sc.Manifest, PagesK, pages)

		return nil
	})

	return []Step{
		index,
		templates,
		resolve,
		build,
	}
}

// StepHeaders writes a headers file from config.
func StepHeaders() Step {
	return StepFunc("headers", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		headersCfg := cfg.Build.Steps.Headers
		if headersCfg == nil {
			return nil
		}

		headers := headersCfg.Headers

		for _, page := range pages {
			if len(page.Headers) == 0 {
				continue
			}
			path := page.Meta.Claim.Canon
			if _, ok := headers[path]; !ok {
				headers[path] = make(map[string]string, len(page.Headers))
			}
			for key, value := range page.Headers {
				headers[path][key] = value
			}
		}

		if len(headers) == 0 {
			return nil
		}

		sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("headers", headersCfg.Output),
			Builder: func(w io.Writer) error {
				for path, kvs := range headers {
					fmt.Fprintf(w, "%s\n", path)
					for k, v := range kvs {
						fmt.Fprintf(w, "  %s: %s\n", k, v)
					}
					fmt.Fprintf(w, "\n")
				}

				return nil
			},
		})

		return nil
	}, "pages:index")
}

// StepRedirects writes a redirects file from config.
func StepRedirects() Step {
	return StepFunc("redirects", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		redirectsCfg := cfg.Build.Steps.Redirects

		if redirectsCfg == nil {
			return nil
		}

		shortSlugForRedirect := func(slug string) string {
			slug = strings.TrimSpace(slug)
			slug = strings.Trim(slug, "/")
			if slug == "" {
				return ""
			}
			if i := strings.LastIndex(slug, "/"); i >= 0 && i < len(slug)-1 {
				return slug[i+1:]
			}
			return slug
		}

		redirects := make([]config.Redirect, 0)
		redirects = append(redirects, redirectsCfg.Redirects...)

		for _, page := range pages {
			if page.Meta.Err != nil {
				continue
			}
			if page.Section != "posts" {
				continue
			}

			shortSlug := shortSlugForRedirect(page.Slug)
			if shortSlug == "" {
				continue
			}

			path, _ := url.JoinPath(cfg.Site.URL, redirectsCfg.Shorten, shortSlug)

			redirects = append(redirects, config.Redirect{
				From:   path,
				To:     page.Meta.Claim.Canon,
				Status: 0,
			})
		}

		if len(redirects) == 0 {
			return nil
		}

		sc.Manifest.Emit(manifest.Artefact{
			Claim: manifest.NewInternalClaim("redirects", redirectsCfg.Output),
			Builder: func(w io.Writer) error {
				for _, redirect := range redirects {
					fmt.Fprintf(w, "%s %s", redirect.From, redirect.To)
					if redirect.Status != 0 {
						fmt.Fprintf(w, " %d", redirect.Status)
					}
					fmt.Fprintf(w, "\n")
				}

				return nil
			},
		})

		return nil
	}, "pages:index")
}

func StepRSS() Step {
	return StepFunc("rss", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)

		cfgRSS := cfg.Build.Steps.RSS
		if cfgRSS == nil {
			return nil
		}

		sc.Manifest.Emit(manifest.TemplateArtefact(manifest.Claim{
			Owner:  "rss",
			Target: cfgRSS.Output,
			Canon:  cfgRSS.Output,
		}, transforms.RSSTemplate, transforms.BuildRSS(pages, site, cfgRSS)))

		return nil
	}, "pages:resolve")
}

func StepSitemap() Step {
	return StepFunc("sitemap", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)

		cfgSitemap := cfg.Build.Steps.Sitemap
		if cfgSitemap == nil {
			return nil
		}

		sc.Manifest.Emit(manifest.TemplateArtefact(manifest.Claim{
			Owner:  "sitemap",
			Target: cfgSitemap.Output,
			Canon:  cfgSitemap.Output,
		}, transforms.SitemapTemplate, transforms.BuildSitemap(pages, site, cfgSitemap)))

		return nil
	}, "pages:resolve")
}
