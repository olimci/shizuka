package urlutil

import "testing"

func TestValidURL(t *testing.T) {
	tests := map[string]string{
		"http://example.com":             "http://example.com",
		"https://example.com/path space": "https://example.com/path%20space",
	}

	for raw, want := range tests {
		got, err := ValidURL(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%s: url = %q, want %q", raw, got, want)
		}
	}
}

func TestValidURLRejectsMissingOrUnsupportedScheme(t *testing.T) {
	for _, raw := range []string{"", "example.com", "mailto:test@example.com"} {
		if _, err := ValidURL(raw); err == nil {
			t.Fatalf("%s: expected validation error", raw)
		}
	}
}
