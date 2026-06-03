package decodeutil

import "testing"

func TestFormatExt(t *testing.T) {
	tests := map[string]Format{
		"toml":  FormatTOML,
		".TOML": FormatTOML,
		"yaml":  FormatYAML,
		"yml":   FormatYAML,
		"json":  FormatJSON,
		"jsonc": FormatJSONC,
	}

	for ext, want := range tests {
		got, ok := FormatExt(ext)
		if !ok {
			t.Fatalf("%s: expected supported format", ext)
		}
		if got != want {
			t.Fatalf("%s: format = %q, want %q", ext, got, want)
		}
	}

	if _, ok := FormatExt("txt"); ok {
		t.Fatal("txt unexpectedly supported")
	}
}

func TestUnmarshalExtDecodesSupportedFormats(t *testing.T) {
	type document struct {
		Title string `toml:"title" yaml:"title" json:"title"`
	}
	tests := map[string][]byte{
		"toml":  []byte(`title = "TOML"`),
		"yaml":  []byte("title: YAML"),
		"json":  []byte(`{"title": "JSON"}`),
		"jsonc": []byte("{// comment\n\"title\": \"JSONC\"}"),
	}

	for ext, data := range tests {
		var doc document
		if err := UnmarshalExt(ext, data, &doc); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		if doc.Title == "" {
			t.Fatalf("%s: decoded empty title", ext)
		}
	}
}

func TestUnmarshalExtRejectsUnsupportedFormat(t *testing.T) {
	var doc struct{}
	if err := UnmarshalExt("txt", []byte("title"), &doc); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
