package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

const (
	ConfigK    = manifest.K[*Config]("config")
	OptionsK   = manifest.K[*Options]("options")
	PagesK     = manifest.K[[]*transforms.Page]("pages")
	SiteK      = manifest.K[transforms.Site]("site")
	TemplatesK = manifest.K[template.Template]("templates")
)

// StepStatic attatches static files
func StepStatic() Step {
	return StepFunc("static", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)

		m := NewMinifier(config.Build.Minify)

		staticSource := config.Build.Steps.Static.Source
		staticDestination := config.Build.Steps.Static.Destination

		files, err := fileutils.WalkFiles(staticSource)
		if err != nil {
			return err
		}

		for _, rel := range files.Values() {
			full := filepath.Join(staticSource, rel)
			target := filepath.Join(staticDestination, rel)
			artefact := makeStatic("static", full, target)
			sc.Manifest.Emit(m.MinifyArtefact(rel, artefact))
		}

		return nil
	})
}

// StepContent builds pages
func StepContent() []Step {
	// build creates the manifest artefacts for the pages
	build := StepFunc("pages:build", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)
		tmpl := manifest.GetAs(sc.Manifest, TemplatesK)

		m := NewMinifier(config.Build.Minify)

		buildTime := time.Now().Format(time.RFC3339)

		siteMeta := transforms.SiteMeta{
			BuildTime: buildTime,
			Dev:       sc.Options.Dev,
		}

		makeArtefact := func(page *transforms.Page, claim manifest.Claim, useTemplate *template.Template, templateName string) manifest.Artefact {
			pageForTemplate := *page
			pageForTemplate.Meta.Template = templateName

			siteForTemplate := site
			siteForTemplate.Meta = siteMeta

			a := manifest.Artefact{
				Claim: claim,
				Builder: func(w io.Writer) error {
					data := transforms.PageTemplate{
						Page: pageForTemplate,
						Site: siteForTemplate,
					}
					return useTemplate.Execute(w, data)
				},
			}

			return m.MinifyArtefact(page.Meta.Target, a)
		}

		for _, page := range pages {
			claim := manifest.Claim{
				Source: page.Meta.Source,
				Target: page.Meta.Target,
				Owner:  "pages:build",
			}

			if page.Meta.Err != nil {
				if errTmpl := lookupErrPage(sc.Options, page.Meta.Err); errTmpl != nil {
					sc.Manifest.Emit(makeArtefact(page, claim, errTmpl, page.Meta.Template))
				}
				continue
			}

			pageTmpl := tmpl.Lookup(page.Meta.Template)
			templateName := page.Meta.Template

			if pageTmpl == nil {
				if page.Meta.Template == "" {
					page.Meta.Err = errors.Join(ErrNoTemplate, errors.New("no template specified"))
					_ = sc.Error(page.Meta.Source, "no template specified", page.Meta.Err)
				} else {
					page.Meta.Err = errors.Join(ErrTemplateNotFound, fmt.Errorf("template %q not found", page.Meta.Template))
					_ = sc.Error(page.Meta.Source, fmt.Sprintf("template %q not found", page.Meta.Template), page.Meta.Err)
				}

				if errTmpl := lookupErrPage(sc.Options, page.Meta.Err); errTmpl != nil {
					sc.Manifest.Emit(makeArtefact(page, claim, errTmpl, page.Meta.Template))
				}
				continue
			}

			pageForTemplate := *page
			templateNameForTemplate := templateName

			artefact := manifest.Artefact{
				Claim: claim,
				Builder: func(w io.Writer) error {
					siteForTemplate := site
					siteForTemplate.Meta = siteMeta

					data := transforms.PageTemplate{
						Page: pageForTemplate,
						Site: siteForTemplate,
					}
					return tmpl.ExecuteTemplate(w, templateNameForTemplate, data)
				},
			}
			sc.Manifest.Emit(m.MinifyArtefact(page.Meta.Target, artefact))
		}

		return nil
	}, "pages:resolve")

	// resolve creates the manifest registry entries for site information.
	resolve := StepFunc("pages:resolve", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		site := transforms.Site{
			Title:       config.Site.Title,
			Description: config.Site.Description,
			URL:         config.Site.URL,
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

		sc.Manifest.Set(string(SiteK), site)

		return nil
	}, "pages:index")

	// index indexes pages and creates the manifest registry entries for page information.
	index := StepFunc("pages:index", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)

		tmpl, err := parseTemplateGlob(config.Build.Steps.Content.TemplateGlob, WithTemplateFuncs(transforms.DefaultTemplateFuncs()))
		if err != nil {
			return fmt.Errorf("failed to parse templates: %w", err)
		}

		sc.Manifest.Set(string(TemplatesK), tmpl)

		contentSource := config.Build.Steps.Content.Source
		contentDestination := config.Build.Steps.Content.Destination
		defaultParams := config.Build.Steps.Content.DefaultParams
		defaultLiteParams := config.Build.Steps.Content.DefaultLiteParams

		md := makeGoldmark(config.Build.Steps.Content.GoldmarkConfig)

		files, err := fileutils.WalkFiles(contentSource)
		if err != nil {
			return err
		}

		pages := make([]*transforms.Page, 0, len(files.Values()))

		for _, rel := range files.Values() {
			source, target, err := makeTarget(contentSource, rel)
			if err != nil {
				sc.Warn(rel, "invalid target path, skipping", err)
				continue
			}
			target = filepath.Join(contentDestination, target)

			if filepath.Ext(source) == ".html" {
				sc.Manifest.Emit(makeStatic("pages:index", source, target))
				continue
			}

			page, err := transforms.BuildPage(source, md)
			if err != nil {
				pageErr := errors.Join(ErrPageBuild, err)
				if sc.Options.Dev {
					pages = append(pages, &transforms.Page{
						Meta: transforms.PageMeta{
							Source: source,
							Target: target,
							Err:    pageErr,
						},
					})
				}

				_ = sc.Error(source, "failed to build page", err)
				continue
			}

			page.Meta.Target = target

			params := maps.Clone(defaultParams)
			liteParams := maps.Clone(defaultLiteParams)
			maps.Copy(params, page.Params)
			maps.Copy(liteParams, page.LiteParams)

			page.Params = params
			page.LiteParams = liteParams

			pages = append(pages, page)
		}

		sc.Manifest.Set(string(PagesK), pages)

		return nil
	})

	return []Step{
		index,
		resolve,
		build,
	}
}

// StepHeaders writes a headers file from config.
func StepHeaders() Step {
	return StepFunc("headers", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		headersCfg := config.Build.Steps.Headers
		pages := manifest.GetAs(sc.Manifest, PagesK)
		if headersCfg == nil && len(pages) == 0 {
			return nil
		}

		headers := make(map[string]map[string]string)
		if headersCfg != nil {
			for path, entries := range headersCfg.Headers {
				headers[path] = maps.Clone(entries)
			}
		}

		for _, page := range pages {
			if len(page.Headers) == 0 {
				continue
			}
			path := targetToPath(page.Meta.Target)
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

		content := renderHeaders(headers)
		artefact := makeTextArtefact("headers", "_headers", content)
		sc.Manifest.Emit(artefact)
		return nil
	}, "pages:index")
}

// StepRedirects writes a redirects file from config.
func StepRedirects() Step {
	return StepFunc("redirects", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		redirectsCfg := config.Build.Steps.Redirects
		pages := manifest.GetAs(sc.Manifest, PagesK)
		if redirectsCfg == nil && len(pages) == 0 {
			return nil
		}

		redirects := make([]Redirect, 0)
		if redirectsCfg != nil {
			redirects = append(redirects, redirectsCfg.Redirects...)
		}

		shortenRoot := ""
		if redirectsCfg != nil {
			shortenRoot = redirectsCfg.Shorten
		}

		for _, page := range pages {
			if strings.TrimSpace(page.Slug) == "" {
				continue
			}
			redirects = append(redirects, Redirect{
				From:   shortenPath(shortenRoot, page.Slug),
				To:     targetToPath(page.Meta.Target),
				Status: 0,
			})
		}

		if len(redirects) == 0 {
			return nil
		}

		content := renderRedirects(redirects)
		artefact := makeTextArtefact("redirects", "_redirects", content)
		sc.Manifest.Emit(artefact)
		return nil
	}, "pages:index")
}
