package build

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
	"github.com/olimci/shizuka/pkg/utils/stack"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

const (
	ConfigK    = manifest.K[*config.Config]("config")
	OptionsK   = manifest.K[*config.Options]("options")
	PagesK     = manifest.K[*transforms.PageTree]("pages")
	SiteK      = manifest.K[*transforms.Site]("site")
	TemplatesK = manifest.K[*template.Template]("templates")
	BuildCtxK  = manifest.K[*BuildCtx]("buildctx")
)

// StepStatic attatches static files
func StepStatic() Step {
	return StepFunc("static", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)

		m := NewMinifier(cfg.Build.Minify)

		sourceRoot, err := cleanFSPath(cfg.Build.Steps.Static.Source)
		if err != nil {
			return fmt.Errorf("static source %q: %w", cfg.Build.Steps.Static.Source, err)
		}
		targetRoot, err := cleanFSPath(cfg.Build.Steps.Static.Destination)
		if err != nil {
			return fmt.Errorf("static destination %q: %w", cfg.Build.Steps.Static.Destination, err)
		}

		files, err := fileutils.WalkFilesFS(sc.SourceFS, sourceRoot)
		if err != nil {
			return err
		}

		for _, rel := range files.Values() {
			source := path.Join(sourceRoot, rel)
			target := path.Join(targetRoot, rel)
			sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceFS, manifest.Claim{
				Owner:  "static",
				Source: source,
				Target: target,
				Canon:  path.Join(targetRoot, rel),
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

		for _, node := range pages.Nodes() {
			claim := manifest.NewPageClaim(node.Path, node.URLPath)

			if node.Error != nil {
				if errTmpl := lookupErrPage(opts.PageErrTemplates, node.Error); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Site:  *site,
						Error: node.Error,
					}).Post(m))
				}

				continue
			}

			page := node.Page
			if page == nil {
				continue
			}
			if page.Draft && !opts.Dev {
				continue
			}

			templateName := strings.TrimSpace(page.Meta.Template)
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
				var err error
				err = errors.Join(ErrTemplateNotFound, fmt.Errorf("template %q not found", templateName))
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
			Params:      maps.Clone(cfg.Site.Params),

			Meta: transforms.SiteMeta{
				ConfigPath: opts.ConfigPath,
				IsDev:      opts.Dev,

				BuildTime:       buildCtx.StartTime,
				BuildTimeString: buildCtx.StartTimestring,
			},
		}

		if cfg.Build.Steps.Content != nil && cfg.Build.Steps.Content.Cascade != nil {
			var cascade func(*transforms.PageNode, map[string]any)
			cascade = func(node *transforms.PageNode, params map[string]any) {
				temp := maps.Clone(params)
				if node.Page != nil {
					maps.Copy(temp, node.Page.Cascade)
					node.Page.Cascade = temp
				}
				for _, child := range node.Children {
					cascade(child, temp)
				}
			}

			cascade(pages.Root, cfg.Build.Steps.Content.Cascade)
		}

		site.Tree = pages

		seenSlugs := make(map[string]string)
		for _, node := range pages.Nodes() {
			if node.Error != nil || node.Page == nil {
				continue
			}

			page := node.Page

			slugSource := page.Slug
			if strings.TrimSpace(slugSource) == "" {
				slugSource = page.Meta.URLPath
			}
			slug, err := transforms.CleanSlug(slugSource)
			if err != nil {
				sc.Error(fmt.Errorf("invalid slug %q: %w", slugSource, err), manifest.NewPageClaim(page.Meta.Source, page.Meta.URLPath))
			} else {
				page.Slug = slug
				if slug != "" {
					if prev, ok := seenSlugs[slug]; ok {
						sc.Error(
							fmt.Errorf("duplicate slug %q (%s, %s)", slug, prev, page.Meta.Source),
							manifest.NewPageClaim(page.Meta.Source, page.Meta.URLPath),
						)
					} else {
						seenSlugs[slug] = page.Meta.Source
					}
				}
			}

			canon, err := canonicalPageURL(site.URL, page.Meta.URLPath)
			if err != nil {
				sc.Error(
					fmt.Errorf("canonical URL from site.url %q and page path %q: %w", site.URL, page.Meta.URLPath, err),
					manifest.NewPageClaim(page.Meta.Source, page.Meta.URLPath),
				)
			} else {
				page.Canon = canon
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

		glob, err := cleanFSGlob(config.Build.Steps.Content.TemplateGlob)
		if err != nil {
			return fmt.Errorf("template glob %q: %w", config.Build.Steps.Content.TemplateGlob, err)
		}

		tmpl, err := parseTemplates(sc.SourceFS, glob, transforms.DefaultTemplateFuncs())
		if err != nil {
			return fmt.Errorf("template glob %q: %w", glob, err)
		}

		manifest.SetAs(sc.Manifest, TemplatesK, tmpl)

		return nil
	})

	// index indexes pages and creates the manifest registry entries for page information.
	index := StepFunc("pages:index", func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, ConfigK)

		root, err := cleanFSPath(cfg.Build.Steps.Content.Source)
		if err != nil {
			return fmt.Errorf("content source %q: %w", cfg.Build.Steps.Content.Source, err)
		}

		tree, err := fileutils.WalkTreeFS(sc.SourceFS, root)
		if err != nil {
			return fmt.Errorf("content source %q: %w", root, err)
		}

		md := cfg.Build.Steps.Content.GoldmarkConfig.Build()

		rootPage := &(transforms.PageNode{
			Path: ".",
		})
		s := stack.New(rootPage)

		tree.Traverse(func(node *fileutils.FSNode, depth int) {
			// directories internally correspond to pagenodes where page=nil
			parent, _ := s.Peek()

			if node.IsDir {
				if node.Path == "." {
					return
				}

				dirNode := &(transforms.PageNode{
					Path:    node.Path,
					URLPath: url2dir(node.Path),
				})
				if ok := parent.AddChild(node.Name, dirNode); !ok {
					sc.Error(fmt.Errorf("duplicate page node %q", node.Name), manifest.NewInternalClaim("pages:index", path.Join(dirNode.URLPath, "index.html")))
				}
				s.Push(dirNode)
				return
			}

			dir, base := path.Split(node.Path)
			ext := path.Ext(base)
			name := strings.TrimSuffix(base, ext)
			source := path.Join(root, node.Path)
			urlPath := path.Join(url2dir(dir), name)
			if name == "index" {
				urlPath = url2dir(dir)
			}
			claim := manifest.NewPageClaim(source, urlPath)

			if ext == ".html" {
				sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceFS, claim))
				return
			}

			page, err := transforms.BuildPageFS(sc.SourceFS, source, md)
			if err != nil {
				sc.Error(err, claim)
			}

			if err == nil {
				params := maps.Clone(cfg.Build.Steps.Content.DefaultParams)
				maps.Copy(params, page.Params)
				page.Params = params
			}

			// If name == index, then the page corresponds to the current directory
			if name == "index" {
				parent.Path = source
				parent.URLPath = url2dir(dir)
				if err != nil {
					parent.Error = err
					return
				}

				page.Meta.URLPath = parent.URLPath
				page.Meta.Target = path.Join(page.Meta.URLPath, "index.html")
				parent.Page = page
				page.Tree = parent
				return
			}

			child := new(transforms.PageNode)
			child.Path = source
			child.URLPath = path.Join(url2dir(dir), name)
			if err != nil {
				child.Error = err
			} else {
				page.Meta.URLPath = child.URLPath
				page.Meta.Target = path.Join(page.Meta.URLPath, "index.html")
				child.Page = page
				page.Tree = child
			}
			if ok := parent.AddChild(name, child); !ok {
				sc.Error(fmt.Errorf("duplicate page node %q", name), manifest.NewInternalClaim("pages:index", path.Join(child.URLPath, "index.html")))
			}
		}, func(node *fileutils.FSNode, depth int) {
			if node.IsDir && node.Path != "." {
				s.Pop()
			}
		})

		pageTree := transforms.NewPageTree(rootPage)
		manifest.SetAs(sc.Manifest, PagesK, pageTree)

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

		for _, page := range pages.Pages() {
			if len(page.Headers) == 0 {
				continue
			}
			path := page.Meta.URLPath
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

		redirects := make([]config.Redirect, 0)
		redirects = append(redirects, redirectsCfg.Redirects...)

		for _, page := range pages.Pages() {
			if page.Section != "posts" {
				continue
			}

			shortSlug := shortSlugForRedirect(page.Slug)
			if shortSlug == "" {
				continue
			}

			shortPath := path.Join(redirectsCfg.Shorten, shortSlug)

			redirects = append(redirects, config.Redirect{
				From:   shortPath,
				To:     ensureLeadingSlash(page.Meta.URLPath),
				Status: 0,
			})
		}

		if len(redirects) == 0 {
			return nil
		}

		seen := make(map[string]string, len(redirects))
		deduped := make([]config.Redirect, 0, len(redirects))
		for _, redirect := range redirects {
			from := strings.TrimSpace(redirect.From)
			to := strings.TrimSpace(redirect.To)

			if prev, ok := seen[from]; ok {
				sc.Error(
					fmt.Errorf("duplicate redirect %q (%s, %s)", from, prev, to),
					manifest.NewInternalClaim("redirects", redirectsCfg.Output),
				)
				continue
			}

			seen[from] = to
			deduped = append(deduped, redirect)
		}
		redirects = deduped

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

		sc.Manifest.Emit(manifest.TemplateArtefact(
			manifest.NewInternalClaim("rss", cfgRSS.Output),
			transforms.RSSTemplate.Get(),
			transforms.BuildRSS(pages.Pages(), site, cfgRSS),
		))

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

		sc.Manifest.Emit(manifest.TemplateArtefact(
			manifest.NewInternalClaim("sitemap", cfgSitemap.Output),
			transforms.SitemapTemplate.Get(),
			transforms.BuildSitemap(pages.Pages(), site, cfgSitemap),
		))

		return nil
	}, "pages:resolve")
}
