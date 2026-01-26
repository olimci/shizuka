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
