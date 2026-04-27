package build

import (
	"path"
	"regexp"
	"strings"

	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

var (
	markdownLinkPattern = regexp.MustCompile(`(!?\[[^\]]*\]\()([^) \t\r\n]+)([^)]*\))`)
	wikilinkPattern     = regexp.MustCompile(`(!?)\[\[([^\]|#]+)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
)

type pageLookup struct {
	exact     map[string]*transforms.Page
	basename  map[string]*transforms.Page
	ambiguous map[string]struct{}
}

func newPageLookup(pages []*transforms.Page) *pageLookup {
	lookup := &pageLookup{
		exact:     make(map[string]*transforms.Page),
		basename:  make(map[string]*transforms.Page),
		ambiguous: make(map[string]struct{}),
	}

	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}

		for _, candidate := range []string{
			page.Slug,
			page.URLPath,
			pathutil.URLPathForContentPath(page.ContentPath),
		} {
			candidate = strings.Trim(strings.TrimSpace(candidate), "/")
			if candidate == "" {
				continue
			}
			lookup.exact[candidate] = page
		}

		rel := page.ContentPath
		if rel == "" {
			continue
		}
		base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		if base == "" {
			continue
		}
		if existing, ok := lookup.basename[base]; !ok {
			lookup.basename[base] = page
		} else if existing != nil && existing.SourcePath != page.SourcePath {
			lookup.ambiguous[base] = struct{}{}
		}
	}

	return lookup
}

func preprocessMarkdown(page *transforms.Page, lookup *pageLookup, wikilinks bool) (string, []transforms.PageLink) {
	body := page.Source.RawBody
	links := make([]transforms.PageLink, 0)

	if wikilinks {
		var wikilinkLinks []transforms.PageLink
		body, wikilinkLinks = rewriteMarkdownWikilinks(body, page, lookup)
		links = append(links, wikilinkLinks...)
	}

	body = rewriteMarkdownBundleLinks(body, page.Assets)
	links = append(links, discoverMarkdownLinks(page.Source.RawBody, page, lookup)...)

	return body, links
}

func rewriteMarkdownBundleLinks(body string, assets map[string]*transforms.PageAsset) string {
	return markdownLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		parts := markdownLinkPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		resolved := resolveBundleURL(parts[2], assets)
		if resolved == "" {
			return match
		}
		return parts[1] + resolved + parts[3]
	})
}

func rewriteMarkdownWikilinks(body string, page *transforms.Page, lookup *pageLookup) (string, []transforms.PageLink) {
	links := make([]transforms.PageLink, 0)

	body = wikilinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		parts := wikilinkPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}

		embed := parts[1] == "!"
		target := strings.TrimSpace(parts[2])
		fragment := strings.TrimSpace(parts[3])
		alias := strings.TrimSpace(parts[4])
		if target == "" {
			return match
		}

		if assetURL, mediaType, ok := resolveWikilinkAsset(page.Assets, target); ok {
			url := assetURL
			if fragment != "" {
				url += "#" + fragment
			}
			if embed && strings.HasPrefix(mediaType, "image/") {
				return "![](" + url + ")"
			}

			label := alias
			if label == "" {
				label = path.Base(target)
			}
			return "[" + label + "](" + url + ")"
		}

		link := transforms.PageLink{
			Source:    page,
			RawTarget: target,
			Fragment:  fragment,
			Label:     alias,
			Embed:     embed,
		}
		if linkedPage, ok := resolvePageTarget(lookup, target); ok {
			link.Target = linkedPage
		}

		if link.Target == nil {
			return match
		}
		links = append(links, link)

		url := "/"
		if link.Target.URLPath != "" {
			url = "/" + strings.Trim(link.Target.URLPath, "/") + "/"
		}
		if fragment != "" {
			url += "#" + fragment
		}

		label := alias
		if label == "" {
			label = link.Target.Title
		}
		if label == "" {
			label = target
		}
		return "[" + label + "](" + url + ")"
	})

	return body, links
}

func discoverMarkdownLinks(body string, page *transforms.Page, lookup *pageLookup) []transforms.PageLink {
	if page == nil || page.Source.Format != transforms.PageSourceFormatMarkdown {
		return nil
	}

	links := make([]transforms.PageLink, 0)

	for _, parts := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		if len(parts) != 4 || strings.HasPrefix(parts[1], "![") {
			continue
		}

		label := markdownLinkLabel(parts[1])
		rawTarget, fragment := splitLinkTarget(parts[2])
		if rawTarget == "" {
			continue
		}

		if resolveBundleURL(rawTarget, page.Assets) != "" {
			continue
		}

		normalizedTarget, ok := normalizeMarkdownPageTarget(page.URLPath, rawTarget)
		if !ok {
			continue
		}

		link := transforms.PageLink{
			Source:    page,
			RawTarget: rawTarget,
			Fragment:  fragment,
			Label:     label,
		}
		if linkedPage, ok := resolvePageTarget(lookup, normalizedTarget); ok {
			link.Target = linkedPage
		}
		if link.Target == nil {
			continue
		}
		links = append(links, link)
	}

	return links
}

func markdownLinkLabel(prefix string) string {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "(")
	prefix = strings.TrimPrefix(prefix, "!")
	if len(prefix) < 2 || prefix[0] != '[' || prefix[len(prefix)-1] != ']' {
		return ""
	}
	return strings.TrimSpace(prefix[1 : len(prefix)-1])
}

func splitLinkTarget(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	fragment := ""
	if idx := strings.Index(raw, "#"); idx >= 0 {
		fragment = strings.TrimSpace(raw[idx+1:])
		raw = raw[:idx]
	}

	if idx := strings.Index(raw, "?"); idx >= 0 {
		raw = raw[:idx]
	}

	return strings.TrimSpace(raw), fragment
}

func normalizeMarkdownPageTarget(pageURLPath, rawTarget string) (string, bool) {
	rawTarget = strings.TrimSpace(rawTarget)
	if rawTarget == "" || rawTarget == "." || rawTarget == ".." {
		return "", false
	}

	lower := strings.ToLower(rawTarget)
	for _, prefix := range []string{"http://", "https://", "mailto:", "tel:", "data:", "javascript:"} {
		if strings.HasPrefix(lower, prefix) {
			return "", false
		}
	}
	if strings.HasPrefix(rawTarget, "#") || strings.HasPrefix(rawTarget, "?") || strings.HasPrefix(rawTarget, "//") {
		return "", false
	}

	resolved := rawTarget
	if strings.HasPrefix(rawTarget, "/") {
		resolved = path.Clean(rawTarget)
	} else {
		resolved = path.Clean(path.Join("/", pageURLPath, rawTarget))
	}

	ext := strings.ToLower(path.Ext(resolved))
	switch ext {
	case "":
	case ".md", ".html":
		resolved = strings.TrimSuffix(resolved, ext)
	default:
		return "", false
	}

	if before, ok := strings.CutSuffix(resolved, "/index"); ok {
		resolved = before
	}
	if resolved == "/index" {
		resolved = "/"
	}

	target := strings.Trim(resolved, "/")
	if target == "." {
		target = ""
	}
	return target, true
}

func resolveWikilinkAsset(assets map[string]*transforms.PageAsset, target string) (string, string, bool) {
	key := path.Clean(strings.TrimPrefix(strings.TrimSpace(target), "./"))
	if key == "." || key == "" || strings.HasPrefix(key, "../") {
		return "", "", false
	}
	asset, ok := assets[key]
	if !ok || asset == nil {
		return "", "", false
	}
	return asset.URL, asset.MediaType, true
}

func resolvePageTarget(lookup *pageLookup, target string) (*transforms.Page, bool) {
	if lookup == nil {
		return nil, false
	}

	target = strings.Trim(strings.TrimSpace(target), "/")
	if target == "" {
		return nil, false
	}

	if exact := lookup.exact[target]; exact != nil {
		return exact, true
	}
	if _, ambiguous := lookup.ambiguous[target]; ambiguous {
		return nil, false
	}
	if basename := lookup.basename[target]; basename != nil {
		return basename, true
	}
	return nil, false
}
