package urlutil

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func ValidURL(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("is required")
	}
	if !(strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")) {
		return "", fmt.Errorf("must start with http:// or https:// (got %q)", raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("is not a valid URL (got %q): %w", raw, err)
	}
	return parsed.String(), nil
}
