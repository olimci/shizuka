package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/pathutil"
)

var htmlAttrPattern = regexp.MustCompile(`\b(href|src)=(?:"([^"]+)"|'([^']+)')`)

func isPageSourceExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".html", ".toml", ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func collectBundleDir(sc *StepContext, page *transforms.Page, bundleDir, root string, files []string, owned, emitted map[string]string, mode, outputRoot string) error {
	for _, file := range files {
		base := path.Base(file)
		if isPageSourceExt(path.Ext(base)) {
			return fmt.Errorf("bundle directory %q contains page source %q", bundleDir, file)
		}

		key := strings.TrimPrefix(strings.TrimPrefix(file, bundleDir), "/")
		if key == "" || key == "." {
			continue
		}

		if err := attachBundleAsset(sc, page, key, path.Join(root, file), owned, emitted, mode, outputRoot); err != nil {
			return err
		}
	}
	return nil
}

func attachBundleAsset(sc *StepContext, page *transforms.Page, key, source string, owned, emitted map[string]string, mode, outputRoot string) error {
	key = strings.TrimPrefix(path.Clean(key), "/")
	if key == "." || strings.HasPrefix(key, "../") {
		return fmt.Errorf("invalid bundle asset key %q", key)
	}

	if owner, exists := owned[source]; exists && owner != page.SourcePath {
		return fmt.Errorf("bundle asset %q claimed by %q and %q", source, owner, page.SourcePath)
	}
	owned[source] = page.SourcePath

	data, err := os.ReadFile(filepath.Join(sc.SourceRoot, filepath.FromSlash(source)))
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:12])

	target := path.Join(page.URLPath, key)
	if mode == "fingerprinted" {
		ext := path.Ext(source)
		target = path.Join(outputRoot, hash+ext)
	}

	asset := &transforms.PageAsset{
		Key:        key,
		Source:     source,
		Target:     target,
		URL:        pathutil.EnsureLeadingSlash(target),
		Hash:       hash,
		Size:       int64(len(data)),
		MediaType:  pageAssetMediaType(source),
		Standalone: mode == "fingerprinted",
	}
	page.Assets[key] = asset

	if prev, ok := emitted[target]; ok {
		if prev == hash {
			return nil
		}
		return fmt.Errorf("bundle output conflict for %q", target)
	}
	emitted[target] = hash
	sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceRoot, manifest.Claim{
		Owner:  "pages:assets",
		Source: source,
		Target: target,
		Canon:  asset.URL,
	}))
	return nil
}

func rewriteHTMLBundleLinks(body string, assets map[string]*transforms.PageAsset) string {
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

		resolved := resolveBundleURL(value, assets)
		if resolved == "" {
			return match
		}

		return parts[1] + "=" + quote + resolved + quote
	})
}

func resolveBundleURL(raw string, assets map[string]*transforms.PageAsset) string {
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

	asset, ok := assets[key]
	if !ok || asset == nil {
		return ""
	}
	return asset.URL
}

func indexContentFiles(files []string) (map[string][]string, map[string][]string) {
	filesByDir := make(map[string][]string)
	descendantsByDir := make(map[string][]string)

	for _, file := range files {
		file = path.Clean(strings.TrimPrefix(strings.TrimSpace(file), "/"))
		if file == "." || file == "" {
			continue
		}

		dir := path.Dir(file)
		if dir == "." {
			dir = ""
		}
		filesByDir[dir] = append(filesByDir[dir], file)

		for parent := dir; parent != ""; {
			descendantsByDir[parent] = append(descendantsByDir[parent], file)
			next := path.Dir(parent)
			if next == "." {
				break
			}
			parent = next
		}
	}

	return filesByDir, descendantsByDir
}

func pageAssetMediaType(source string) string {
	typ := mime.TypeByExtension(strings.ToLower(path.Ext(source)))
	if typ == "" {
		return "application/octet-stream"
	}
	return typ
}
