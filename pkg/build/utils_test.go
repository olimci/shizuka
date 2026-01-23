package build

import (
	"path"
	"path/filepath"
	"testing"
)

func TestCleanFSPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "simple path",
			input:   "content/posts/hello.md",
			want:    "content/posts/hello.md",
			wantErr: false,
		},
		{
			name:    "path with leading slash",
			input:   "/content/posts/hello.md",
			want:    "",
			wantErr: true, // Leading slash makes it absolute
		},
		{
			name:    "path with dot",
			input:   "./content/posts/hello.md",
			want:    "content/posts/hello.md",
			wantErr: false,
		},
		{
			name:    "path with multiple slashes",
			input:   "content//posts///hello.md",
			want:    "content/posts/hello.md",
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			input:   "content/../etc/passwd",
			want:    "etc/passwd", // path.Clean resolves this to "etc/passwd" which is allowed
			wantErr: false,
		},
		{
			name:    "path traversal at start",
			input:   "../etc/passwd",
			want:    "",
			wantErr: true,
		},
		{
			name:    "absolute path",
			input:   "/etc/passwd",
			want:    "",
			wantErr: true,
		},
		{
			name:    "windows absolute path",
			input:   "C:/Windows/System32",
			want:    "C:/Windows/System32", // Only absolute on Windows, treated as relative on Unix
			wantErr: false,
		},
		{
			name:    "empty path",
			input:   "",
			want:    "",
			wantErr: true, // Empty path returns error
		},
		{
			name:    "dot only",
			input:   ".",
			want:    ".",
			wantErr: false,
		},
		{
			name:    "backslashes converted",
			input:   "content\\posts\\hello.md",
			want:    path.Clean(filepath.ToSlash("content\\posts\\hello.md")),
			wantErr: false,
		},
		{
			name:    "url encoded path traversal",
			input:   "content/%2e%2e/etc/passwd",
			want:    "content/%2e%2e/etc/passwd", // URL encoding is not decoded
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanFSPath(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanFSPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("cleanFSPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanFSGlob(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "simple glob",
			input:   "content/**/*.md",
			want:    "content/**/*.md",
			wantErr: false,
		},
		{
			name:    "glob with leading slash",
			input:   "/content/**/*.md",
			want:    "",
			wantErr: true, // Leading slash makes it absolute
		},
		{
			name:    "glob with wildcards",
			input:   "templates/**/*.{html,tmpl}",
			want:    "templates/**/*.{html,tmpl}",
			wantErr: false,
		},
		{
			name:    "glob with path traversal",
			input:   "content/../**/*.md",
			want:    "content/../**/*.md",
			wantErr: false,
		},
		{
			name:    "glob with absolute path",
			input:   "/etc/**/*",
			want:    "",
			wantErr: true,
		},
		{
			name:    "single star glob",
			input:   "static/*",
			want:    "static/*",
			wantErr: false,
		},
		{
			name:    "question mark glob",
			input:   "content/post?.md",
			want:    "content/post?.md",
			wantErr: false,
		},
		{
			name:    "character class glob",
			input:   "content/[abc]*.md",
			want:    "content/[abc]*.md",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanFSGlob(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanFSGlob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("cleanFSGlob() = %q, want %q", got, tt.want)
			}
		})
	}
}
