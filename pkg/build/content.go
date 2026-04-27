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
	"slices"
	"strings"

	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/transforms"
	"github.com/olimci/shizuka/pkg/utils/fileutil"
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
	return emitPageAsset(sc, page, asset, owned, emitted)
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

func pageBundleSources(page *transforms.Page, filesByDir, descendantsByDir map[string][]string) ([]string, error) {
	if page == nil {
		return nil, nil
	}

	rel := page.ContentPath
	if rel == "" {
		return nil, fmt.Errorf("content path missing for %q", page.SourcePath)
	}

	base := path.Base(rel)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	dir := path.Dir(rel)
	if dir == "." {
		dir = ""
	}

	sources := make([]string, 0)
	for _, child := range filesByDir[dir] {
		if child == rel {
			continue
		}

		childName := path.Base(child)
		childExt := path.Ext(childName)
		if isPageSourceExt(childExt) {
			continue
		}
		childStem := strings.TrimSuffix(childName, childExt)
		if childStem != stem {
			continue
		}

		sources = append(sources, child)
	}

	bundleDir := path.Join(dir, stem)
	if bundleDir != "" {
		for _, file := range descendantsByDir[bundleDir] {
			base := path.Base(file)
			if isPageSourceExt(path.Ext(base)) {
				return nil, fmt.Errorf("bundle directory %q contains page source %q", bundleDir, file)
			}
			sources = append(sources, file)
		}
	}

	slices.Sort(sources)
	return slices.Compact(sources), nil
}

func pageBundleInputs(root string, sources []string) (map[string]fileFingerprint, error) {
	inputs := make(map[string]fileFingerprint, len(sources))
	for _, source := range sources {
		fingerprint, err := statFingerprint(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil {
			return nil, err
		}
		inputs[source] = fingerprint
	}
	return inputs, nil
}

func sameBundleInputs(a, b map[string]fileFingerprint) bool {
	if len(a) != len(b) {
		return false
	}

	for key, fp := range a {
		other, ok := b[key]
		if !ok || !fp.Equal(other) {
			return false
		}
	}
	return true
}

func buildPageAssets(sc *StepContext, page *transforms.Page, contentRootRel string, sources []string, mode, outputRoot string) (map[string]*transforms.PageAsset, error) {
	assets := make(map[string]*transforms.PageAsset, len(sources))
	for _, sourceRel := range sources {
		key := path.Base(sourceRel)
		dir := path.Dir(page.ContentPath)
		if dir == "." {
			dir = ""
		}
		base := path.Base(page.ContentPath)
		stem := strings.TrimSuffix(base, path.Ext(base))
		bundleDir := path.Join(dir, stem)
		if bundleDir != "" && strings.HasPrefix(sourceRel, bundleDir+"/") {
			key = strings.TrimPrefix(sourceRel, bundleDir+"/")
		}

		source := path.Join(contentRootRel, sourceRel)
		asset, err := buildPageAsset(sc, page, key, source, mode, outputRoot)
		if err != nil {
			return nil, err
		}
		assets[key] = asset
	}
	return assets, nil
}

func buildPageAsset(sc *StepContext, page *transforms.Page, key, source, mode, outputRoot string) (*transforms.PageAsset, error) {
	key = strings.TrimPrefix(path.Clean(key), "/")
	if key == "." || strings.HasPrefix(key, "../") {
		return nil, fmt.Errorf("invalid bundle asset key %q", key)
	}

	data, err := os.ReadFile(filepath.Join(sc.SourceRoot, filepath.FromSlash(source)))
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:12])

	target := path.Join(page.URLPath, key)
	if mode == "fingerprinted" {
		ext := path.Ext(source)
		target = path.Join(outputRoot, hash+ext)
	}

	return &transforms.PageAsset{
		Key:        key,
		Source:     source,
		Target:     target,
		URL:        pathutil.EnsureLeadingSlash(target),
		Hash:       hash,
		Size:       int64(len(data)),
		MediaType:  pageAssetMediaType(source),
		Standalone: mode == "fingerprinted",
	}, nil
}

func emitPageAsset(sc *StepContext, page *transforms.Page, asset *transforms.PageAsset, owned, emitted map[string]string) error {
	if page == nil || asset == nil {
		return nil
	}

	if owner, exists := owned[asset.Source]; exists && owner != page.SourcePath {
		return fmt.Errorf("bundle asset %q claimed by %q and %q", asset.Source, owner, page.SourcePath)
	}
	owned[asset.Source] = page.SourcePath

	copied := *asset
	copied.Owner = page
	page.Assets[copied.Key] = &copied

	if prev, ok := emitted[copied.Target]; ok {
		if prev == copied.Hash {
			return nil
		}
		return fmt.Errorf("bundle output conflict for %q", copied.Target)
	}
	emitted[copied.Target] = copied.Hash
	sc.Manifest.Emit(manifest.StaticArtefact(sc.SourceRoot, manifest.Claim{
		Owner:  "pages:assets",
		Source: copied.Source,
		Target: copied.Target,
		Canon:  copied.URL,
	}))
	return nil
}

func attachPageFileMeta(page *transforms.Page, source string) {
	if page == nil {
		return
	}

	info, err := fileutil.Info(source)
	if err != nil {
		return
	}

	page.File = transforms.PageFileMeta{
		Available: true,
		Created:   info.Created,
		Updated:   info.Updated,
		Size:      info.Size,
	}
	if !page.Source.HasCreated && !info.Created.IsZero() {
		page.Created = info.Created
	}
	if !page.Source.HasUpdated && !info.Updated.IsZero() {
		page.Updated = info.Updated
	}
	if !page.Updated.IsZero() {
		page.PubDate = page.Updated
	} else if !page.Created.IsZero() {
		page.PubDate = page.Created
	}
}
