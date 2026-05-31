package pathutil

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

type ResolvedPath struct {
	Root string
	Raw  string
	Path string
}

func ResolvePath(root, raw string) (ResolvedPath, error) {
	rel, err := CleanContentPath(raw)
	if err != nil {
		return ResolvedPath{}, err
	}

	return ResolvedPath{
		Root: filepath.Clean(root),
		Raw:  raw,
		Path: filepath.Clean(filepath.Join(root, filepath.FromSlash(rel))),
	}, nil
}

func ResolveGlob(root, raw string) (ResolvedPath, error) {
	rel, err := CleanContentGlob(raw)
	if err != nil {
		return ResolvedPath{}, err
	}

	return ResolvedPath{
		Root: filepath.Clean(root),
		Raw:  raw,
		Path: filepath.Clean(filepath.Join(root, filepath.FromSlash(rel))),
	}, nil
}

func JoinSlashRel(root, rel string) string {
	root = filepath.ToSlash(filepath.Clean(root))
	rel = filepath.ToSlash(rel)
	if root == "." || root == "" {
		return rel
	}
	return path.Join(root, rel)
}

func CleanContentPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("absolute paths are not supported: %q", p)
	}
	p = filepath.ToSlash(p)
	if p == "." {
		return ".", nil
	}
	if strings.HasPrefix(p, "/") {
		return "", fmt.Errorf("path must be relative: %q", p)
	}
	if cleaned := path.Clean(p); cleaned != p {
		return "", fmt.Errorf("path must be clean (got %q, want %q)", p, cleaned)
	}
	if EscapesRoot(p) {
		return "", fmt.Errorf("path %q escapes source root", p)
	}
	return p, nil
}

func CleanContentGlob(pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("empty glob")
	}
	if filepath.IsAbs(pattern) {
		return "", fmt.Errorf("absolute globs are not supported: %q", pattern)
	}
	pattern = filepath.ToSlash(pattern)
	if strings.HasPrefix(pattern, "/") {
		return "", fmt.Errorf("glob must be relative: %q", pattern)
	}
	if EscapesRoot(pattern) {
		return "", fmt.Errorf("glob %q escapes source root", pattern)
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
	target = strings.ToLower(target)
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

func RoutePathForContentPath(rel string) (string, error) {
	rel = path.Clean(rel)
	if rel == "." || rel == "" {
		return "/", nil
	}

	dir, base := path.Split(rel)
	name := strings.TrimSuffix(base, path.Ext(base))
	var route string
	if name == "index" {
		var err error
		route, err = routePathDir(dir)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		dir, err = routePathDir(dir)
		if err != nil {
			return "", err
		}
		name = normalizePathSegment(name)
		if name == "" {
			return "", fmt.Errorf("content path %q has no routeable filename segment", rel)
		}
		if dir == "" {
			route = name
		} else {
			route = path.Join(dir, name)
		}
	}

	if route == "" {
		return "/", nil
	}
	return "/" + strings.Trim(route, "/") + "/", nil
}

func ValidateRoutePath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("route path is empty")
	}
	if strings.TrimSpace(raw) != raw {
		return "", fmt.Errorf("route path has surrounding whitespace (got %q)", raw)
	}
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("route path must start with / (got %q)", raw)
	}
	if !strings.HasSuffix(raw, "/") {
		return "", fmt.Errorf("route path must end with / (got %q)", raw)
	}
	if strings.ContainsAny(raw, "\\?#") {
		return "", fmt.Errorf("route path must not contain any of: \\, ?, # (got %q)", raw)
	}
	if raw == "/" {
		return "/", nil
	}
	trimmed := strings.Trim(raw, "/")
	if cleaned := path.Clean(trimmed); cleaned != trimmed {
		return "", fmt.Errorf("route path must be clean (got %q, want /%s/)", raw, cleaned)
	}

	for seg := range strings.SplitSeq(trimmed, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("route path contains invalid segment %q (got %q)", seg, raw)
		}
		for _, r := range seg {
			if unicode.IsSpace(r) || unicode.IsControl(r) {
				return "", fmt.Errorf("route path contains whitespace/control character (got %q)", raw)
			}
			if !isUnreservedURLRune(r) {
				return "", fmt.Errorf("route path contains non-url-safe character %q (got %q)", r, raw)
			}
		}
	}

	return raw, nil
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

func OutputPathForRoutePath(routePath string) string {
	routePath = strings.Trim(routePath, "/")
	if routePath == "" {
		return "index.html"
	}
	return path.Join(routePath, "index.html")
}

func routePathDir(dir string) (string, error) {
	dir = path.Clean(strings.TrimSpace(dir))
	if dir == "." || dir == "/" || dir == "" {
		return "", nil
	}
	dir = strings.TrimPrefix(dir, "/")

	parts := strings.Split(dir, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		segment := normalizePathSegment(part)
		if segment == "" {
			return "", fmt.Errorf("content path directory %q has no routeable segment for %q", dir, part)
		}
		out = append(out, segment)
	}
	return strings.Join(out, "/"), nil
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
	if _, err := ValidateRoutePath("/" + strings.Trim(raw, "/") + "/"); err == nil {
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
