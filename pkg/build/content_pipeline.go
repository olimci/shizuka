package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutils"
)

var (
	markdownLinkPattern = regexp.MustCompile(`(!?\[[^\]]*\]\()([^) \t\r\n]+)([^)]*\))`)
	htmlAttrPattern     = regexp.MustCompile(`\b(href|src)=(?:"([^"]+)"|'([^']+)')`)
	obsidianLinkPattern = regexp.MustCompile(`(!?)\[\[([^\]|#]+)(?:#([^\]|]+))?(?:\|([^\]]+))?\]\]`)
)

func isPageSourceExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".html", ".toml", ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func collectBundleDir(sc *StepContext, page *transforms.Page, dirNode *fileutils.FSNode, root string, owned, emitted map[string]string, mode, outputRoot string) error {
	var walk func(*fileutils.FSNode) error
	walk = func(node *fileutils.FSNode) error {
		for _, child := range node.Children {
			if child == nil {
				continue
			}
			if child.IsDir {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}

			if isPageSourceExt(path.Ext(child.Name)) {
				return fmt.Errorf("bundle directory %q contains page source %q", dirNode.Path, child.Path)
			}

			key := strings.TrimPrefix(strings.TrimPrefix(child.Path, dirNode.Path), "/")

			if err := attachBundleAsset(sc, page, key, path.Join(root, child.Path), owned, emitted, mode, outputRoot); err != nil {
				return err
			}
		}
		return nil
	}

	return walk(dirNode)
}

func attachBundleAsset(sc *StepContext, page *transforms.Page, key, source string, owned, emitted map[string]string, mode, outputRoot string) error {
	key = strings.TrimPrefix(path.Clean(key), "/")
	if key == "." || strings.HasPrefix(key, "../") {
		return fmt.Errorf("invalid bundle asset key %q", key)
	}

	if owner, exists := owned[source]; exists && owner != page.Meta.Source {
		return fmt.Errorf("bundle asset %q claimed by %q and %q", source, owner, page.Meta.Source)
	}
	owned[source] = page.Meta.Source

	data, err := fs.ReadFile(sc.SourceFS, source)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:12])

	target := path.Join(page.Meta.URLPath, key)
	if mode == "fingerprinted" {
		ext := path.Ext(source)
		target = path.Join(outputRoot, hash+ext)
	}

	asset := &transforms.PageAsset{
		Key:        key,
		Source:     source,
		Target:     target,
		URL:        ensureLeadingSlash(target),
		Hash:       hash,
		Size:       int64(len(data)),
		MediaType:  transforms.PageAssetMediaType(source),
		Standalone: mode == "fingerprinted",
	}
	page.Bundle.Assets[key] = asset

	if prev, ok := emitted[target]; ok {
		if prev == hash {
			return nil
		}
		return fmt.Errorf("bundle output conflict for %q", target)
	}
	emitted[target] = hash
	sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceFS, manifest.Claim{
		Owner:  "pages:assets",
		Source: source,
		Target: target,
		Canon:  asset.URL,
	}))
	return nil
}

func rewriteMarkdownBundleLinks(body string, bundle transforms.PageBundle) string {
	return markdownLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		parts := markdownLinkPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		resolved := resolveBundleURL(parts[2], bundle)
		if resolved == "" {
			return match
		}
		return parts[1] + resolved + parts[3]
	})
}

func rewriteHTMLBundleLinks(body string, bundle transforms.PageBundle) string {
	return htmlAttrPattern.ReplaceAllStringFunc(body, func(match string) string {
		parts := htmlAttrPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}

		value := parts[2]
		quote := `"`
		if value == "" {
			value = parts[3]
			quote = `'`
		}

		resolved := resolveBundleURL(value, bundle)
		if resolved == "" {
			return match
		}

		return parts[1] + "=" + quote + resolved + quote
	})
}

func rewriteObsidianLinks(body string, page *transforms.Page, pages *transforms.PageTree, contentRoot string) string {
	return obsidianLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		parts := obsidianLinkPattern.FindStringSubmatch(match)
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

		if assetURL, mediaType, ok := resolveWikiAsset(page.Bundle, target); ok {
			url := withFragment(assetURL, fragment)
			if embed {
				if strings.HasPrefix(mediaType, "image/") {
					return "![](" + url + ")"
				}
				label := alias
				if label == "" {
					label = path.Base(target)
				}
				return "[" + label + "](" + url + ")"
			}
			label := alias
			if label == "" {
				label = path.Base(target)
			}
			return "[" + label + "](" + url + ")"
		}

		if linkedPage, ok := resolveWikiPage(pages, contentRoot, target); ok {
			url := withFragment(pagePublicURL(linkedPage.Meta.URLPath), fragment)
			label := alias
			if label == "" {
				label = linkedPage.Title
			}
			if label == "" {
				label = target
			}
			if embed {
				return "[" + label + "](" + url + ")"
			}
			return "[" + label + "](" + url + ")"
		}

		return match
	})
}

func resolveBundleURL(raw string, bundle transforms.PageBundle) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "#") {
		return ""
	}

	lower := strings.ToLower(raw)
	for _, prefix := range []string{"http://", "https://", "mailto:", "tel:", "data:", "javascript:"} {
		if strings.HasPrefix(lower, prefix) {
			return ""
		}
	}

	key := path.Clean(strings.TrimPrefix(raw, "./"))
	if key == "." || strings.HasPrefix(key, "../") {
		return ""
	}

	asset, ok := bundle.Assets[key]
	if !ok || asset == nil {
		return ""
	}
	return asset.URL
}

func resolveWikiAsset(bundle transforms.PageBundle, target string) (string, string, bool) {
	key := path.Clean(strings.TrimPrefix(strings.TrimSpace(target), "./"))
	if key == "." || key == "" || strings.HasPrefix(key, "../") {
		return "", "", false
	}
	asset, ok := bundle.Assets[key]
	if !ok || asset == nil {
		return "", "", false
	}
	return asset.URL, asset.MediaType, true
}

func resolveWikiPage(pages *transforms.PageTree, contentRoot, target string) (*transforms.Page, bool) {
	target = strings.Trim(strings.TrimSpace(target), "/")
	if target == "" {
		return nil, false
	}

	var (
		exact     *transforms.Page
		basename  *transforms.Page
		ambiguous bool
	)

	for _, node := range pages.Nodes() {
		if node == nil || node.Page == nil || node.Error != nil {
			continue
		}
		page := node.Page

		if page.Slug == target || page.Meta.URLPath == target {
			return page, true
		}

		if rel, err := relContentPath(contentRoot, page.Meta.Source); err == nil {
			if transforms.URLPathForContentPath(rel) == target {
				exact = page
			}

			base := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
			if base == target {
				if basename == nil {
					basename = page
				} else if basename.Meta.Source != page.Meta.Source {
					ambiguous = true
				}
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

func relContentPath(root, source string) (string, error) {
	root = path.Clean(root)
	source = path.Clean(source)
	if root == "." {
		return source, nil
	}
	prefix := root + "/"
	if !strings.HasPrefix(source, prefix) {
		return "", fmt.Errorf("source %q is not within %q", source, root)
	}
	return strings.TrimPrefix(source, prefix), nil
}

func rawMarkdownTarget(urlPath string) string {
	urlPath = strings.Trim(path.Clean(urlPath), "/")
	if urlPath == "" || urlPath == "." {
		return "index.md"
	}
	return urlPath + ".md"
}

func pagePublicURL(urlPath string) string {
	urlPath = strings.Trim(strings.TrimSpace(urlPath), "/")
	if urlPath == "" {
		return "/"
	}
	return "/" + urlPath + "/"
}

func withFragment(url, fragment string) string {
	if fragment == "" {
		return url
	}
	return url + "#" + fragment
}

func addRawVariants(sc *StepContext, page *transforms.Page) {
	cfg := manifest.GetAs(sc.Manifest, ConfigK)
	if page.Source.Kind == transforms.PageSourceKindMarkdown && cfg.Build.Steps.Content.Raw.Markdown {
		sc.Manifest.Emit(manifest.TextArtefact(manifest.Claim{
			Owner:  "pages:raw",
			Source: page.Meta.Source,
			Target: rawMarkdownTarget(page.Meta.URLPath),
			Canon:  ensureLeadingSlash(rawMarkdownTarget(page.Meta.URLPath)),
		}, page.Source.Doc))
	}
}
