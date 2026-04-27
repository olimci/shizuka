package transforms

import (
	"errors"
	"strings"
	"testing"
)

func TestExtractFrontmatterSupportsYAMLTOMLAndJSON(t *testing.T) {
	tests := []struct {
		name     string
		doc      string
		wantType string
		wantBody string
		wantSlug string
	}{
		{
			name: "yaml",
			doc: `---
slug: hello
title: Hello
---
body
`,
			wantType: "yaml",
			wantBody: "body\n",
			wantSlug: "hello",
		},
		{
			name: "toml",
			doc: `+++
slug = "hello"
title = "Hello"
+++
body
`,
			wantType: "toml",
			wantBody: "body\n",
			wantSlug: "hello",
		},
		{
			name:     "json",
			doc:      "\xef\xbb\xbf{\"slug\":\"hello\",\"title\":\"Hello\"}\nbody\n",
			wantType: "json",
			wantBody: "body\n",
			wantSlug: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := ExtractFrontmatter([]byte(tt.doc))
			if err != nil {
				t.Fatalf("ExtractFrontmatter() error = %v", err)
			}
			if fm == nil || fm.Slug != tt.wantSlug {
				t.Fatalf("frontmatter = %#v, want slug %q", fm, tt.wantSlug)
			}
			if string(body) != tt.wantBody {
				t.Fatalf("body = %q, want %q", body, tt.wantBody)
			}

			kind, _, _, _ := detectFrontmatter(trimBOM([]byte(tt.doc)))
			if kind != tt.wantType {
				t.Fatalf("detectFrontmatter() kind = %q, want %q", kind, tt.wantType)
			}
		})
	}
}

func TestExtractFrontmatterHandlesMissingAndInvalidContent(t *testing.T) {
	doc := []byte("plain body")
	fm, body, err := ExtractFrontmatter(doc)
	if err != nil {
		t.Fatalf("ExtractFrontmatter() error = %v", err)
	}
	if fm != nil {
		t.Fatalf("frontmatter = %#v, want nil", fm)
	}
	if string(body) != "plain body" {
		t.Fatalf("body = %q, want %q", body, "plain body")
	}

	_, _, err = ExtractFrontmatter([]byte("---\nslug: [oops\n---\nbody"))
	if !errors.Is(err, ErrFailedToParseFrontmatter) {
		t.Fatalf("ExtractFrontmatter(invalid) error = %v, want ErrFailedToParseFrontmatter", err)
	}
}

func TestScanJSONObjectPrefixHandlesEmbeddedBracesAndBodyOffset(t *testing.T) {
	doc := []byte("{\"title\":\"{Hello}\",\"nested\":{\"ok\":true}}\r\nbody")
	kind, start, end, bodyStart := scanJSONObjectPrefix(doc)

	if kind != "json" || start != 0 {
		t.Fatalf("scanJSONObjectPrefix() = %q, %d, want json, 0", kind, start)
	}
	if string(doc[end:bodyStart]) != "\r\n" {
		t.Fatalf("newline span = %q, want CRLF", doc[end:bodyStart])
	}
	if string(doc[bodyStart:]) != "body" {
		t.Fatalf("body = %q, want %q", doc[bodyStart:], "body")
	}
}

func TestFrontmatterHelpers(t *testing.T) {
	if !hasPrefixAtLineStart([]byte("--- \nbody"), []byte("---")) {
		t.Fatal("hasPrefixAtLineStart() = false, want true")
	}
	if got := lineEnd([]byte("a\nb"), 0); got != 2 {
		t.Fatalf("lineEnd() = %d, want 2", got)
	}
	if got := skipSingleLineEnding([]byte("x\r\ny"), 1); got != 3 {
		t.Fatalf("skipSingleLineEnding() = %d, want 3", got)
	}
	if got := trimBOM([]byte("\xef\xbb\xbfbody")); string(got) != "body" {
		t.Fatalf("trimBOM() = %q, want %q", got, "body")
	}
	if kind, _, _, _ := detectFrontmatter([]byte(" ---\nbody")); kind != "" {
		t.Fatalf("detectFrontmatter(non-leading fence) = %q, want empty", kind)
	}
	if !strings.Contains(ErrUnknownFrontmatterType.Error(), "unknown frontmatter") {
		t.Fatalf("ErrUnknownFrontmatterType = %v, want descriptive error", ErrUnknownFrontmatterType)
	}
}
