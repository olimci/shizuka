package build

import (
	"errors"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
	"github.com/olimci/shizuka/pkg/utils/tmplutil"
)

type pageRenderRequest struct {
	Claim        manifest.Claim
	TemplateName string
	Templates    *template.Template
	Page         transforms.PageTmpl
	Site         transforms.SiteTmpl
	Pagination   *transforms.PaginationTmpl
	Owners       []string
	Minifier     manifest.PostProcessor
}

func renderPageTemplate(sc *StepContext, req pageRenderRequest) error {
	if err := validatePageRenderRequest(req); err != nil {
		sc.Error(err, req.Claim)
		return nil
	}

	owners := append(slices.Clone(req.Owners), req.TemplateName)
	rendered, err := executePageTemplate(req)
	if err == nil {
		return sc.Manifest.Emit(manifest.TextArtefact(req.Claim, rendered).Post(req.Minifier))
	}

	if tmplutil.IsDiscard(err) {
		return nil
	}

	var paginationErr *paginationEffect
	if !errors.As(err, &paginationErr) {
		sc.Error(err, req.Claim)
		return nil
	}

	return renderPaginationEffect(sc, req, owners, *paginationErr)
}

func validatePageRenderRequest(req pageRenderRequest) error {
	if req.TemplateName == "" {
		return fmt.Errorf("template name is empty")
	}
	if req.Templates == nil {
		return fmt.Errorf("template set is nil")
	}
	if req.Templates.Lookup(req.TemplateName) == nil {
		return fmt.Errorf("template %q not found", req.TemplateName)
	}
	if slices.Contains(req.Owners, req.TemplateName) {
		chain := append(slices.Clone(req.Owners), req.TemplateName)
		return fmt.Errorf("%w: %s", ErrPaginationCycle, strings.Join(chain, " -> "))
	}
	return nil
}

func executePageTemplate(req pageRenderRequest) (string, error) {
	var buf strings.Builder
	err := req.Templates.ExecuteTemplate(&buf, req.TemplateName, transforms.PageTemplate{
		Page:       req.Page,
		Site:       req.Site,
		Pagination: req.Pagination,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderPaginationEffect(sc *StepContext, req pageRenderRequest, owners []string, effect paginationEffect) error {
	if req.Templates.Lookup(effect.PageTemplate) == nil {
		sc.Error(fmt.Errorf("template %q not found", effect.PageTemplate), req.Claim)
		return nil
	}
	if slices.Contains(owners, effect.PageTemplate) {
		chain := append(slices.Clone(owners), effect.PageTemplate)
		sc.Error(fmt.Errorf("%w: %s", ErrPaginationCycle, strings.Join(chain, " -> ")), req.Claim)
		return nil
	}
	if effect.RootTemplate != "" {
		if err := renderPageTemplate(sc, pageRenderRequest{
			Claim:        req.Claim,
			TemplateName: effect.RootTemplate,
			Templates:    req.Templates,
			Page:         req.Page,
			Site:         req.Site,
			Owners:       owners,
			Minifier:     req.Minifier,
		}); err != nil {
			return err
		}
	}

	pages, err := buildPaginationPages(req.Page.Path, effect)
	if err != nil {
		sc.Error(err, req.Claim)
		return nil
	}

	for _, page := range pages {
		claim := manifest.NewPageClaim(req.Claim.Source, page.Route)
		if req.Claim.Owner != "" {
			claim = claim.Own(req.Claim.Owner)
		}
		if err := renderPageTemplate(sc, pageRenderRequest{
			Claim:        claim,
			TemplateName: effect.PageTemplate,
			Templates:    req.Templates,
			Page:         pageTmplForRoute(req.Page, req.Site, page.Route),
			Site:         req.Site,
			Pagination:   &page.Data,
			Owners:       owners,
			Minifier:     req.Minifier,
		}); err != nil {
			return err
		}
	}
	return nil
}

func pageTmplForRoute(page transforms.PageTmpl, site transforms.SiteTmpl, route string) transforms.PageTmpl {
	page.Path = route
	if canon, err := pathutil.CanonicalPageURL(site.URL, route); err == nil {
		page.Canon = canon
	} else {
		page.Canon = ""
	}
	return page
}
