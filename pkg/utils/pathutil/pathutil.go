package pathutil

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

type ResolvedPath struct {
	Root string
	Raw  string
	Path string
}

func ResolvePath(root, raw string) (ResolvedPath, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResolvedPath{}, fmt.Errorf("empty path")
	}

	resolved := raw
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, filepath.FromSlash(raw))
	}

	resolved = filepath.Clean(resolved)

	return ResolvedPath{
		Root: filepath.Clean(root),
		Raw:  raw,
		Path: resolved,
	}, nil
}

func ResolveGlob(root, raw string) (ResolvedPath, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResolvedPath{}, fmt.Errorf("empty glob")
	}

	resolved := raw
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, filepath.FromSlash(raw))
	}

	resolved = filepath.Clean(resolved)

	return ResolvedPath{
		Root: filepath.Clean(root),
		Raw:  raw,
		Path: resolved,
	}, nil
}

func CleanContentPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute paths are not supported: %q", p)
	}
	p = filepath.ToSlash(p)
	p = path.Clean(p)
	if p == "." {
		return ".", nil
	}
	if EscapesRoot(p) {
		return "", fmt.Errorf("path %q escapes source root", p)
	}
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ".", nil
	}
	return p, nil
}

func CleanContentGlob(pattern string) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", fmt.Errorf("empty glob")
	}
	if filepath.IsAbs(pattern) {
		return "", fmt.Errorf("absolute globs are not supported: %q", pattern)
	}
	pattern = filepath.ToSlash(pattern)
	if EscapesRoot(pattern) {
		return "", fmt.Errorf("glob %q escapes source root", pattern)
	}
	pattern = strings.TrimPrefix(pattern, "/")
	if pattern == "" {
		return "", fmt.Errorf("empty glob")
	}
	return pattern, nil
}

func EscapesRoot(p string) bool {
	for {
		if p == ".." {
			return true
		}
		dir, base := path.Split(p)
		if base == ".." {
			return true
		}
		if dir == "" || dir == "/" {
			return false
		}
		p = strings.TrimSuffix(dir, "/")
	}
}

func EnsureLeadingSlash(target string) string {
	if target == "" {
		return "/"
	}
	if strings.HasPrefix(target, "/") || IsExternalURL(target) {
		return target
	}
	return "/" + target
}

func IsExternalURL(target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	switch {
	case strings.HasPrefix(target, "http://"):
		return true
	case strings.HasPrefix(target, "https://"):
		return true
	case strings.HasPrefix(target, "mailto:"):
		return true
	case strings.HasPrefix(target, "tel:"):
		return true
	default:
		return false
	}
}

func CanonicalPageURL(siteURL, pagePath string) (string, error) {
	canon, err := url.JoinPath(siteURL, pagePath)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(canon, "/") {
		canon += "/"
	}
	return canon, nil
}

func ShortSlugForRedirect(slug string) string {
	slug = strings.TrimSpace(slug)
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return ""
	}
	if i := strings.LastIndex(slug, "/"); i >= 0 && i < len(slug)-1 {
		return slug[i+1:]
	}
	return slug
}

func URLPathForContentPath(rel string) string {
	rel = path.Clean(strings.TrimSpace(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." || rel == "" {
		return ""
	}

	dir, base := path.Split(rel)
	name := strings.TrimSuffix(base, path.Ext(base))
	if name == "index" {
		return cleanContentDir(dir)
	}
	dir = cleanContentDir(dir)
	name = normalizePathSegment(name)
	if dir == "" {
		return name
	}
	return path.Join(dir, name)
}

func CleanSlug(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	if strings.ContainsAny(s, "\\?#") {
		return "", fmt.Errorf("slug must not contain any of: \\, ?, # (got %q)", raw)
	}

	s = strings.Trim(s, "/")
	if s == "" {
		return "", nil
	}

	if cleaned := path.Clean(s); cleaned != s {
		return "", fmt.Errorf("slug must be clean (got %q, want %q)", raw, cleaned)
	}

	for seg := range strings.SplitSeq(s, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("slug contains invalid segment %q (got %q)", seg, raw)
		}
		for _, r := range seg {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return "", fmt.Errorf("slug contains whitespace/control character (got %q)", raw)
			}
			if !isUnreservedURLRune(r) {
				return "", fmt.Errorf("slug contains non-url-safe character %q (got %q)", r, raw)
			}
		}
	}

	return s, nil
}

func CleanURLPath(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "/" {
		return "", nil
	}
	return CleanSlug(strings.Trim(s, "/"))
}

func RelPathWithin(root, source string) (string, error) {
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

func OutputPathForURLPath(urlPath string) string {
	return path.Join(strings.Trim(urlPath, "/"), "index.html")
}

func cleanContentDir(dir string) string {
	dir = path.Clean(strings.TrimSpace(dir))
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	dir = strings.TrimPrefix(dir, "/")

	parts := strings.Split(dir, "/")
	parts = slices.DeleteFunc(parts, func(part string) bool { return part == "" || part == "." })
	for i, part := range parts {
		parts[i] = normalizePathSegment(part)
	}
	return strings.Join(parts, "/")
}

func isUnreservedURLRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
		return true
	case r >= 'A' && r <= 'Z':
		return true
	case r >= '0' && r <= '9':
		return true
	}
	switch r {
	case '-', '.', '_', '~':
		return true
	default:
		return false
	}
}

func normalizePathSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "." || raw == ".." {
		return ""
	}
	if _, err := CleanSlug(raw); err == nil {
		return raw
	}

	var b strings.Builder
	lastSep := false

	for _, r := range strings.ToLower(raw) {
		switch {
		case isUnreservedURLRune(r):
			b.WriteRune(r)
			lastSep = false
		case unicode.IsSpace(r) || unicode.IsControl(r):
			if b.Len() == 0 || lastSep {
				continue
			}
			b.WriteByte('-')
			lastSep = true
		default:
			if b.Len() == 0 || lastSep {
				continue
			}
			b.WriteByte('-')
			lastSep = true
		}
	}

	return strings.Trim(b.String(), "-")
}
