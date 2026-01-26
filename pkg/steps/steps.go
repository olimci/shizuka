package steps

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"maps"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/steps/keys"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
	"github.com/olimci/shizuka/pkg/utils/stack"
)

var (
	ErrNoTemplate       = errors.New("no template found")
	ErrTemplateNotFound = errors.New("template not found")
)

var (
	IDStatic         = StepID{Owner: "shizuka", Name: "static"}
	IDPagesIndex     = StepID{Owner: "shizuka", Name: "pages", Sub: "index"}
	IDPagesRender    = StepID{Owner: "shizuka", Name: "pages", Sub: "render"}
	IDPagesTemplates = StepID{Owner: "shizuka", Name: "pages", Sub: "templates"}
	IDPagesResolve   = StepID{Owner: "shizuka", Name: "pages", Sub: "resolve"}
	IDPagesBuild     = StepID{Owner: "shizuka", Name: "pages", Sub: "build"}
	IDHeaders        = StepID{Owner: "shizuka", Name: "headers"}
	IDRedirects      = StepID{Owner: "shizuka", Name: "redirects"}
	IDRSS            = StepID{Owner: "shizuka", Name: "rss"}
	IDSitemap        = StepID{Owner: "shizuka", Name: "sitemap"}
)
var StepStatic = StepFunc(IDStatic, func(sc *StepContext) error {
	cfg := manifest.GetAs(sc.Manifest, keys.Config)

	m := transforms.NewMinifier(cfg.Build.Minify)

	sourceRoot, err := cleanFSPath(cfg.Build.Steps.Static.Source)
	if err != nil {
		return fmt.Errorf("static source: %w", err)
	}
	targetRoot, err := cleanFSPath(cfg.Build.Steps.Static.Destination)
	if err != nil {
		return fmt.Errorf("static destination: %w", err)
	}

	files, err := fileutils.WalkFilesFS(sc.SourceFS, sourceRoot)
	if err != nil {
		return err
	}

	for _, rel := range files.Values() {
		source := path.Join(sourceRoot, rel)
		target := path.Join(targetRoot, rel)
		sc.Manifest.Emit(manifest.StaticArtefact(manifest.Claim{
			Owner:  "static",
			Source: source,
			Target: target,
			Canon:  path.Join(targetRoot, rel),
		}).Post(m))
	}

	return nil
})

// StepContent builds pages.
func StepContent() []Step {
	return []Step{
		StepPagesIndex(),
		StepPagesRender(),
		StepPagesTemplates(),
		StepPagesResolve(),
		StepPagesBuild(),
	}
}

// StepPagesBuild creates the manifest artefacts for pages.
func StepPagesBuild() Step {
	return StepFunc(IDPagesBuild, func(sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, keys.Options)
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)
		site := manifest.GetAs(sc.Manifest, keys.Site)
		tmpl := manifest.GetAs(sc.Manifest, keys.Templates)

		m := transforms.NewMinifier(cfg.Build.Minify)

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

			if tmpl.Lookup(page.Meta.Template) == nil {
				var err error
				if page.Meta.Template == "" {
					err = errors.Join(ErrNoTemplate, errors.New("no template specified"))
					sc.Errorf(err, "page %s: no template specified", claim)
				} else {
					err = errors.Join(ErrTemplateNotFound, fmt.Errorf("template %q not found", page.Meta.Template))
					sc.Errorf(err, "template %q not found", page.Meta.Template)
				}

				if errTmpl := lookupErrPage(opts.PageErrTemplates, err); errTmpl != nil {
					sc.Manifest.Emit(manifest.TemplateArtefact(claim, errTmpl, transforms.PageTemplate{
						Page:  *page,
						Site:  *site,
						Error: err,
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
	}).WithDeps(IDPagesRender, IDPagesResolve, IDPagesTemplates).
		WithReads(string(keys.Pages), string(keys.Site), string(keys.Templates))
}

// StepPagesResolve creates the manifest registry entries for site information.
func StepPagesResolve() Step {
	return StepFunc(IDPagesResolve, func(sc *StepContext) error {
		opts := manifest.GetAs(sc.Manifest, keys.Options)
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)
		buildCtx := manifest.GetAs(sc.Manifest, keys.BuildMeta)

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
		redirectsCfg := cfg.Build.Steps.Redirects

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
				sc.Errorf(err, "page %s: invalid slug %q", page.Meta.Source, slugSource)
			} else {
				page.Slug = slug
				if slug != "" {
					if prev, ok := seenSlugs[slug]; ok {
						sc.Errorf(fmt.Errorf("duplicate slug %q (%s, %s)", slug, prev, page.Meta.Source), "duplicate slug %q", slug)
					} else {
						seenSlugs[slug] = page.Meta.Source
					}
				}
			}

			if redirectsCfg != nil && redirectsCfg.Shorten != "" && page.Section == "posts" {
				shortSlug := shortSlugForRedirect(page.Slug)
				if shortSlug != "" {
					canon, err := url.JoinPath(site.URL, redirectsCfg.Shorten, shortSlug)
					if err != nil {
						sc.Errorf(err, "page %s: failed to build canonical url", page.Meta.Source)
					} else {
						page.Canon = canon
					}
				}
			}
			if page.Canon == "" {
				canon, err := url.JoinPath(site.URL, page.Meta.URLPath)
				if err != nil {
					sc.Errorf(err, "page %s: failed to build canonical url", page.Meta.Source)
				} else if !strings.HasSuffix(canon, "/") {
					page.Canon = canon + "/"
				} else {
					page.Canon = canon
				}
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

		manifest.SetAs(sc.Manifest, keys.Site, site)

		return nil
	}).WithDeps(IDPagesRender).WithWrites(string(keys.Pages), string(keys.Site))
}

// StepPagesTemplates parses the page templates.
func StepPagesTemplates() Step {
	return StepFunc(IDPagesTemplates, func(sc *StepContext) error {
		config := manifest.GetAs(sc.Manifest, keys.Config)

		glob, err := cleanFSGlob(config.Build.Steps.Content.TemplateGlob)
		if err != nil {
			return fmt.Errorf("template glob: %w", err)
		}

		tmpl, err := parseTemplates(sc.SourceFS, glob, transforms.DefaultTemplateFuncs())
		if err != nil {
			return fmt.Errorf("failed to parse templates: %w", err)
		}

		manifest.SetAs(sc.Manifest, keys.Templates, tmpl)

		return nil
	}).WithWrites(string(keys.Templates))
}

// StepPagesRender renders page bodies from content.
func StepPagesRender() Step {
	return StepFunc(IDPagesRender, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)

		md := cfg.Build.Steps.Content.GoldmarkConfig.Build()

		for _, node := range pages.Nodes() {
			switch node.Page.BodyType {
			case "markdown":
				var buf strings.Builder

				if err := md.Convert(node.Page.BodyRaw, &buf); err != nil {
					sc.Errorf(err, "failed to parse markdown from %q", node.URLPath)
					node.Error = err
					continue
				}

				node.Page.Body = template.HTML(buf.String())
			}
		}

		return nil
	}).WithDeps(IDPagesIndex).WithWrites(string(keys.Pages))
}

// StepPagesIndex indexes pages and creates the manifest registry entries for page information.
func StepPagesIndex() Step {
	return StepFunc(IDPagesIndex, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)

		root, err := cleanFSPath(cfg.Build.Steps.Content.Source)
		if err != nil {
			return fmt.Errorf("content source: %w", err)
		}

		tree, err := fileutils.WalkTreeFS(sc.SourceFS, root)
		if err != nil {
			return fmt.Errorf("failed to walk files: %w", err)
		}

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
					sc.Errorf(fmt.Errorf("duplicate page node %q", node.Name), "duplicate page node %q", node.Name)
				}
				s.Push(dirNode)
				return
			}

			dir, base := path.Split(node.Path)
			ext := path.Ext(base)
			name := strings.TrimSuffix(base, ext)
			source := path.Join(root, node.Path)

			if ext == ".html" {
				var url string
				if name == "index" {
					url = url2dir(dir)
				} else {
					url = path.Join(url2dir(dir), name)
				}
				sc.Manifest.Emit(manifest.StaticArtefact(manifest.NewPageClaim(source, url)))
				return
			}

			page, err := transforms.BuildPage(sc.SourceFS, source)
			if err != nil {
				// if we get an error, emit an error and create an empty page node where Error is set
				sc.Error(err, "failed to build page")
			}

			if err == nil {
				params := maps.Clone(cfg.Build.Steps.Content.DefaultParams)
				maps.Copy(params, page.Params)
				page.Params = params
			}

			// If name == index, then the page corresponds to the current directory
			if name == "index" {
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
			child.Path = node.Path
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
				sc.Errorf(fmt.Errorf("duplicate page node %q", name), "duplicate page node %q", name)
			}
		}, func(node *fileutils.FSNode, depth int) {
			if node.IsDir && node.Path != "." {
				s.Pop()
			}
		})

		pageTree := transforms.NewPageTree(rootPage)
		manifest.SetAs(sc.Manifest, keys.Pages, pageTree)

		return nil
	}).WithWrites(string(keys.Pages))
}

// StepHeaders writes a headers file from config.
func StepHeaders() Step {
	return StepFunc(IDHeaders, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)

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
			maps.Copy(headers[path], page.Headers)
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
	}).WithDeps(IDPagesResolve).WithReads(string(keys.Pages))
}

// StepRedirects writes a redirects file from config.
func StepRedirects() Step {
	return StepFunc(IDRedirects, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)

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
	}).WithDeps(IDPagesResolve).WithReads(string(keys.Pages))
}

func StepRSS() Step {
	return StepFunc(IDRSS, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)
		site := manifest.GetAs(sc.Manifest, keys.Site)

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
	}).WithDeps(IDPagesResolve).WithReads(string(keys.Pages), string(keys.Site))
}

func StepSitemap() Step {
	return StepFunc(IDSitemap, func(sc *StepContext) error {
		cfg := manifest.GetAs(sc.Manifest, keys.Config)
		pages := manifest.GetAs(sc.Manifest, keys.Pages)
		site := manifest.GetAs(sc.Manifest, keys.Site)

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
	}).WithDeps(IDPagesResolve).WithReads(string(keys.Pages), string(keys.Site))
}
