package transforms

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

func firstNonzero[T comparable](values ...T) T {
	zero := *new(T)
	for _, value := range values {
		if value != zero {
			return value
		}
	}
	return zero
}

// URLPathForContentPath maps a content source path such as "posts/hello.md" or
// "posts/hello/index.md" to the page URL path used by the build pipeline.
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
	return path.Join(cleanContentDir(dir), name)
}

func cleanContentDir(dir string) string {
	dir = path.Clean(strings.TrimSpace(dir))
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	return strings.TrimPrefix(dir, "/")
}

// CleanSlug normalizes and validates a slug.
//
// A slug is a URL path without a leading or trailing slash. It may contain
// multiple segments separated by "/".
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

	// Reject anything that would be rewritten by path.Clean to avoid ambiguity.
	if cleaned := path.Clean(s); cleaned != s {
		return "", fmt.Errorf("slug must be clean (got %q, want %q)", raw, cleaned)
	}

	for _, seg := range strings.Split(s, "/") {
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

// RFC 3986 unreserved characters: ALPHA / DIGIT / "-" / "." / "_" / "~"
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
