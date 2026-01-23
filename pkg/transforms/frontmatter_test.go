package transforms

import (
	"strings"
	"testing"
	"time"
)

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTitle   string
		wantBody    string
		wantErr     bool
		description string
	}{
		{
			name: "yaml frontmatter with dashes",
			input: `---
title: "Hello World"
description: "A test post"
date: 2024-01-15
tags:
  - golang
  - testing
---

This is the body content.`,
			wantTitle:   "Hello World",
			wantBody:    "This is the body content.",
			description: "A test post",
			wantErr:     false,
		},
		{
			name: "toml frontmatter with plusses",
			input: `+++
title = "TOML Post"
description = "Using TOML"
date = 2024-01-15T00:00:00Z
tags = ["toml", "test"]
+++

TOML body content.`,
			wantTitle:   "TOML Post",
			wantBody:    "TOML body content.",
			description: "Using TOML",
			wantErr:     false,
		},
		{
			name: "json frontmatter",
			input: `{
  "title": "JSON Post",
  "description": "Using JSON",
  "date": "2024-01-15T00:00:00Z",
  "tags": ["json", "test"]
}

---

JSON body content.`,
			wantTitle:   "JSON Post",
			wantBody:    "---\n\nJSON body content.",
			description: "Using JSON",
			wantErr:     false,
		},
		{
			name: "no frontmatter",
			input: `This is just plain content with no frontmatter.

It should still parse.`,
			wantTitle:   "",
			wantBody:    "",
			description: "",
			wantErr:     true, // ExtractFrontmatter returns error when no frontmatter found
		},
		{
			name: "empty frontmatter",
			input: `---
---

Body after empty frontmatter.`,
			wantTitle:   "",
			wantBody:    "Body after empty frontmatter.",
			description: "",
			wantErr:     false,
		},
		{
			name: "frontmatter with BOM",
			input: "\xef\xbb\xbf---\ntitle: \"BOM Test\"\n---\n\nContent with BOM.",
			wantTitle:   "BOM Test",
			wantBody:    "Content with BOM.",
			description: "",
			wantErr:     false,
		},
		{
			name: "malformed yaml",
			input: `---
title: "Test
description: missing quote
---

Body`,
			wantTitle:   "",
			wantBody:    "",
			description: "",
			wantErr:     true,
		},
		{
			name: "frontmatter only",
			input: `---
title: "Only Frontmatter"
---`,
			wantTitle:   "Only Frontmatter",
			wantBody:    "",
			description: "",
			wantErr:     false,
		},
		{
			name: "multiline body",
			input: `---
title: "Test"
---

Paragraph 1

Paragraph 2

Paragraph 3`,
			wantTitle:   "Test",
			wantBody:    "Paragraph 1\n\nParagraph 2\n\nParagraph 3",
			description: "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := ExtractFrontmatter([]byte(tt.input))

			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if fm.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", fm.Title, tt.wantTitle)
			}

			bodyStr := strings.TrimSpace(string(body))
			wantBody := strings.TrimSpace(tt.wantBody)
			if bodyStr != wantBody {
				t.Errorf("Body = %q, want %q", bodyStr, wantBody)
			}

			if tt.description != "" && fm.Description != tt.description {
				t.Errorf("Description = %q, want %q", fm.Description, tt.description)
			}
		})
	}
}

func TestFrontmatterDates(t *testing.T) {
	input := `---
title: "Date Test"
date: 2024-01-15T10:30:00Z
updated: 2024-01-20T15:45:00Z
---

Content`

	fm, _, err := ExtractFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("ExtractFrontmatter() error = %v", err)
	}

	if fm.Date.IsZero() {
		t.Error("Date should not be zero")
	}

	expectedDate := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !fm.Date.Equal(expectedDate) {
		t.Errorf("Date = %v, want %v", fm.Date, expectedDate)
	}

	if fm.Updated.IsZero() {
		t.Error("Updated should not be zero")
	}

	expectedUpdated := time.Date(2024, 1, 20, 15, 45, 0, 0, time.UTC)
	if !fm.Updated.Equal(expectedUpdated) {
		t.Errorf("Updated = %v, want %v", fm.Updated, expectedUpdated)
	}
}

func TestFrontmatterTags(t *testing.T) {
	input := `---
title: "Tags Test"
tags:
  - golang
  - testing
  - static-site
---

Content`

	fm, _, err := ExtractFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("ExtractFrontmatter() error = %v", err)
	}

	expectedTags := []string{"golang", "testing", "static-site"}
	if len(fm.Tags) != len(expectedTags) {
		t.Errorf("Tags length = %d, want %d", len(fm.Tags), len(expectedTags))
		return
	}

	for i, tag := range fm.Tags {
		if tag != expectedTags[i] {
			t.Errorf("Tag[%d] = %q, want %q", i, tag, expectedTags[i])
		}
	}
}

func TestFrontmatterParams(t *testing.T) {
	input := `---
title: "Params Test"
params:
  author: "John Doe"
  category: "tutorials"
  featured: true
  views: 1234
---

Content`

	fm, _, err := ExtractFrontmatter([]byte(input))
	if err != nil {
		t.Fatalf("ExtractFrontmatter() error = %v", err)
	}

	if fm.Params == nil {
		t.Fatal("Params should not be nil")
	}

	if author, ok := fm.Params["author"].(string); !ok || author != "John Doe" {
		t.Errorf("Params[author] = %v, want %q", fm.Params["author"], "John Doe")
	}

	if category, ok := fm.Params["category"].(string); !ok || category != "tutorials" {
		t.Errorf("Params[category] = %v, want %q", fm.Params["category"], "tutorials")
	}

	if featured, ok := fm.Params["featured"].(bool); !ok || !featured {
		t.Errorf("Params[featured] = %v, want true", fm.Params["featured"])
	}

	if views, ok := fm.Params["views"].(int); !ok || views != 1234 {
		t.Errorf("Params[views] = %v, want 1234", fm.Params["views"])
	}
}
