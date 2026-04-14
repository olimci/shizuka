package build

import (
	"path"
	"regexp"
	"strings"

	"github.com/olimci/shizuka/pkg/transforms"
)

var (
	markdownLinkPattern = regexp.MustCompile(`(!?\[[^\]]*\]\()([^) \t\r\n]+)([^)]*\))`)
	wikilinkPattern     = regexp.MustCompile(`(!?)\[\[([^\]|#]+)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
)

func preprocessMarkdown(page *transforms.Page, pages []*transforms.Page, wikilinks bool) (string, []transforms.PageLink) {
	body := page.Source.RawBody
	links := make([]transforms.PageLink, 0)

	if wikilinks {
		var wikilinkLinks []transforms.PageLink
		body, wikilinkLinks = rewriteMarkdownWikilinks(body, page, pages)
		links = append(links, wikilinkLinks...)
	}

	body = rewriteMarkdownBundleLinks(body, page.Assets)
	links = append(links, discoverMarkdownLinks(page.Source.RawBody, page, pages)...)

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

func rewriteMarkdownWikilinks(body string, page *transforms.Page, pages []*transforms.Page) (string, []transforms.PageLink) {
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
			RawTarget: target,
			Fragment:  fragment,
			Label:     alias,
			Embed:     embed,
		}
		if linkedPage, ok := resolvePageTarget(pages, target); ok {
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

func discoverMarkdownLinks(body string, page *transforms.Page, pages []*transforms.Page) []transforms.PageLink {
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
			RawTarget: rawTarget,
			Fragment:  fragment,
			Label:     label,
		}
		if linkedPage, ok := resolvePageTarget(pages, normalizedTarget); ok {
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

func resolvePageTarget(pages []*transforms.Page, target string) (*transforms.Page, bool) {
	target = strings.Trim(strings.TrimSpace(target), "/")
	if target == "" {
		return nil, false
	}

	var (
		exact     *transforms.Page
		basename  *transforms.Page
		ambiguous bool
	)

	for _, page := range pages {
		if page == nil || page.HasError() {
			continue
		}

		if page.Slug == target || page.URLPath == target {
			return page, true
		}

		rel := page.ContentPath
		if rel == "" {
			continue
		}

		if transforms.URLPathForContentPath(rel) == target {
			exact = page
		}

		base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		if base == target {
			if basename == nil {
				basename = page
			} else if basename.SourcePath != page.SourcePath {
				ambiguous = true
			}
		}
	}

	if exact != nil {
		return exact, true
	}
	if basename != nil && !ambiguous {
		return basename, true
	}
	return nil, false
}
