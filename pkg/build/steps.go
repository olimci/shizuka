package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"slices"
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
	// internal keys
	ConfigK  = manifest.K[*Config]("config")
	OptionsK = manifest.K[*Options]("options")

	// transform keys
	PagesK = manifest.K[map[string]*transforms.PageData]("pages")
	SiteK  = manifest.K[transforms.Site]("site")
)

func StepStatic() Step {
	return StepFunc("static", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)

		m := newMinifier(config.Build.Transforms.Minify)

		files, err := fileutils.WalkFiles(config.Build.StaticDir)
		if err != nil {
			return err
		}

		for _, rel := range files.Values() {
			full := filepath.Join(config.Build.StaticDir, rel)
			artefact := makeStatic("static", full, rel)
			sc.Manifest.Emit(minifyArtefact(m, rel, artefact))
		}

		return nil
	})
}

func StepContent() Step {
	build := StepFunc("pages:build", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)
		site := manifest.GetAs(sc.Manifest, SiteK)

		tmpl, err := parseTemplateGlob(config.Build.TemplatesGlob)
		if err != nil {
			return fmt.Errorf("failed to parse templates: %w", err)
		}

		m := newMinifier(config.Build.Transforms.Minify)

		// Build time for SiteMeta
		buildTime := time.Now().Format(time.RFC3339)

		// Create SiteMeta once for all pages
		siteMeta := transforms.SiteMeta{
			BuildTime: buildTime,
			Dev:       sc.Options.Dev,
		}

		makeArtefact := func(page *transforms.PageData, claim manifest.Claim, useTemplate *template.Template, templateName string) manifest.Artefact {
			a := manifest.Artefact{
				Claim: claim,
				Builder: func(w io.Writer) error {
					data := transforms.PageTemplate{
						Page: page.Page,
						Site: site,
						Meta: transforms.PageMeta{
							Source:   page.Source,
							Target:   page.Target,
							Template: templateName,
						},
						SiteMeta: siteMeta,
					}
					return useTemplate.Execute(w, data)
				},
			}

			return minifyArtefact(m, page.Target, a)
		}

		for _, page := range pages {
			claim := manifest.Claim{
				Source: page.Source,
				Target: page.Target,
				Owner:  "pages:build",
			}

			// Check if template exists
			pageTmpl := tmpl.Lookup(page.Template)
			templateName := page.Template

			if pageTmpl == nil {
				// Template not found - try fallback
				fallback := sc.Options.FallbackTemplate()
				if fallback != nil {
					// Use fallback template
					if page.Template == "" {
						sc.Warn(page.Source, "no template specified, using fallback", nil)
					} else {
						sc.Warnf(page.Source, "template %q not found, using fallback", page.Template)
					}
					artefact := makeArtefact(page, claim, fallback, page.Template)
					sc.Manifest.Emit(minifyArtefact(m, page.Target, artefact))
					continue
				}

				// No fallback - report error
				if page.Template == "" {
					if err := sc.Error(page.Source, "no template specified", ErrNoTemplate); err != nil {
						continue // Skip this page, but don't abort build
					}
					continue
				} else {
					if err := sc.Error(page.Source, fmt.Sprintf("template %q not found", page.Template), ErrTemplateNotFound); err != nil {
						continue
					}
					continue
				}
			}

			// Template found - create artefact with the named template
			artefact := manifest.Artefact{
				Claim: claim,
				Builder: func(w io.Writer) error {
					data := transforms.PageTemplate{
						Page: page.Page,
						Site: site,
						Meta: transforms.PageMeta{
							Source:   page.Source,
							Target:   page.Target,
							Template: templateName,
						},
						SiteMeta: siteMeta,
					}
					return tmpl.ExecuteTemplate(w, templateName, data)
				},
			}
			sc.Manifest.Emit(minifyArtefact(m, page.Target, artefact))
		}

		return nil
	}, "pages:resolve")

	resolve := StepFunc("pages:resolve", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)
		pages := manifest.GetAs(sc.Manifest, PagesK)

		site := transforms.Site{
			Title:       config.Site.Title,
			Description: config.Site.Description,
			URL:         config.Site.URL,
		}

		for _, page := range pages {
			if page.Page.Featured {
				site.Collections.Featured = append(site.Collections.Featured, page.Page.Lite())
			}

			if page.Page.Draft {
				site.Collections.Drafts = append(site.Collections.Drafts, page.Page.Lite())
			}

			site.Collections.All = append(site.Collections.All, page.Page.Lite())
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
	})

	return StepFunc("pages:index", func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, ConfigK)

		md := MakeGoldmark(config.Build.Goldmark)

		files, err := fileutils.WalkFiles(config.Build.ContentDir)
		if err != nil {
			return err
		}

		pages := make(map[string]*transforms.PageData)

		for _, rel := range files.Values() {
			source, target, err := makeTarget(config.Build.ContentDir, rel)
			if err != nil {
				sc.Warn(rel, "invalid target path, skipping", err)
				continue
			}

			if filepath.Ext(source) == ".html" {
				sc.Manifest.Emit(makeStatic("pages:index", source, target))
				continue
			}

			page, err := transforms.BuildPage(source, md)
			if err != nil {
				if err := sc.Error(source, "failed to build page", err); err != nil {
					continue // Skip this page in lenient mode
				}
				continue
			}

			page.Target = target

			pages[rel] = page
		}

		sc.Manifest.Set(string(PagesK), pages)

		sc.Defer(resolve)
		sc.Defer(build)

		return nil
	})
}
