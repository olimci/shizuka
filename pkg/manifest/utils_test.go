package manifest

import (
	"io"
	"strings"
	"testing"
)

func TestMakeArtefacts(t *testing.T) {
	tests := []struct {
		name            string
		artefacts       []Artefact
		wantCount       int
		wantConflicts   int
		wantConflictKey string
	}{
		{
			name: "no conflicts",
			artefacts: []Artefact{
				{
					Claim: Claim{Target: "index.html", Owner: "pages"},
					Builder: func(w io.Writer) error {
						_, err := w.Write([]byte("page 1"))
						return err
					},
				},
				{
					Claim: Claim{Target: "about.html", Owner: "pages"},
					Builder: func(w io.Writer) error {
						_, err := w.Write([]byte("page 2"))
						return err
					},
				},
			},
			wantCount:     2,
			wantConflicts: 0,
		},
		{
			name: "conflict on same target",
			artefacts: []Artefact{
				{
					Claim: Claim{Target: "index.html", Owner: "pages"},
					Builder: func(w io.Writer) error {
						_, err := w.Write([]byte("page 1"))
						return err
					},
				},
				{
					Claim: Claim{Target: "index.html", Owner: "static"},
					Builder: func(w io.Writer) error {
						_, err := w.Write([]byte("static file"))
						return err
					},
				},
			},
			wantCount:       1,
			wantConflicts:   1,
			wantConflictKey: "index.html",
		},
		{
			name: "multiple conflicts",
			artefacts: []Artefact{
				{
					Claim: Claim{Target: "index.html", Owner: "pages:index"},
				},
				{
					Claim: Claim{Target: "index.html", Owner: "static"},
				},
				{
					Claim: Claim{Target: "style.css", Owner: "pages:styles"},
				},
				{
					Claim: Claim{Target: "style.css", Owner: "static"},
				},
			},
			wantCount:     2,
			wantConflicts: 2,
		},
		{
			name: "three-way conflict",
			artefacts: []Artefact{
				{
					Claim: Claim{Target: "index.html", Owner: "pages"},
				},
				{
					Claim: Claim{Target: "index.html", Owner: "static"},
				},
				{
					Claim: Claim{Target: "index.html", Owner: "redirects"},
				},
			},
			wantCount:       1,
			wantConflicts:   1,
			wantConflictKey: "index.html",
		},
		{
			name:          "empty artefacts",
			artefacts:     []Artefact{},
			wantCount:     0,
			wantConflicts: 0,
		},
		{
			name: "different paths no conflict",
			artefacts: []Artefact{
				{
					Claim: Claim{Target: "posts/post-1.html", Owner: "pages"},
				},
				{
					Claim: Claim{Target: "posts/post-2.html", Owner: "pages"},
				},
				{
					Claim: Claim{Target: "static/style.css", Owner: "static"},
				},
			},
			wantCount:     3,
			wantConflicts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artefacts, conflicts := makeArtefacts(tt.artefacts)

			if len(artefacts) != tt.wantCount {
				t.Errorf("makeArtefacts() got %d artefacts, want %d", len(artefacts), tt.wantCount)
			}

			if len(conflicts) != tt.wantConflicts {
				t.Errorf("makeArtefacts() got %d conflicts, want %d", len(conflicts), tt.wantConflicts)
			}

			if tt.wantConflicts > 0 && tt.wantConflictKey != "" {
				if _, ok := conflicts[tt.wantConflictKey]; !ok {
					t.Errorf("expected conflict on %q, but not found", tt.wantConflictKey)
				}
			}
		})
	}
}

func TestMakeArtefactsConflictOwners(t *testing.T) {
	artefacts := []Artefact{
		{Claim: Claim{Target: "index.html", Owner: "pages:index"}},
		{Claim: Claim{Target: "index.html", Owner: "static:files"}},
		{Claim: Claim{Target: "index.html", Owner: "redirects:rewrite"}},
	}

	_, conflicts := makeArtefacts(artefacts)

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	conflictOwners, ok := conflicts["index.html"]
	if !ok {
		t.Fatal("expected conflict on index.html")
	}

	expectedOwners := []string{"pages:index", "static:files", "redirects:rewrite"}
	if len(conflictOwners) != len(expectedOwners) {
		t.Errorf("conflict has %d owners, want %d", len(conflictOwners), len(expectedOwners))
	}

	for _, expected := range expectedOwners {
		found := false
		for _, owner := range conflictOwners {
			if owner.Owner == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected owner %q not found in conflict", expected)
		}
	}
}

func TestManifestDirs(t *testing.T) {
	tests := []struct {
		name      string
		paths     []string
		wantCount int
		wantPaths []string
	}{
		{
			name: "simple paths",
			paths: []string{
				"index.html",
				"about.html",
			},
			wantCount: 1, // Root directory is always included
			wantPaths: []string{"."},
		},
		{
			name: "nested paths",
			paths: []string{
				"posts/post-1.html",
				"posts/post-2.html",
			},
			wantCount: 2,
			wantPaths: []string{".", "posts"},
		},
		{
			name: "deeply nested paths",
			paths: []string{
				"blog/2024/01/post.html",
			},
			wantCount: 4,
			wantPaths: []string{".", "blog", "blog/2024", "blog/2024/01"},
		},
		{
			name: "multiple branches",
			paths: []string{
				"blog/posts/post-1.html",
				"docs/guide/intro.html",
				"static/css/style.css",
			},
			wantCount: 7,
			wantPaths: []string{".", "blog", "blog/posts", "docs", "docs/guide", "static", "static/css"},
		},
		{
			name: "duplicate directories",
			paths: []string{
				"posts/post-1.html",
				"posts/post-2.html",
				"posts/draft/draft-1.html",
			},
			wantCount: 3,
			wantPaths: []string{".", "posts", "posts/draft"},
		},
		{
			name:      "empty paths",
			paths:     []string{},
			wantCount: 1,
			wantPaths: []string{"."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artefacts := make(map[string]ArtefactBuilder, len(tt.paths))
			for _, p := range tt.paths {
				artefacts[p] = nil
			}

			dirs := manifestDirs(artefacts)

			if dirs.Len() != tt.wantCount {
				t.Errorf("manifestDirs() created %d directories, want %d", dirs.Len(), tt.wantCount)
			}

			for _, wantPath := range tt.wantPaths {
				if !dirs.Has(wantPath) {
					t.Errorf("expected directory %q not found", wantPath)
				}
			}
		})
	}
}

func TestManifestDirsIgnoresUnsafePaths(t *testing.T) {
	artefacts := map[string]ArtefactBuilder{
		"safe/path/file.html": nil,
		"../escape/file.html": nil,
		"/absolute/file.html": nil,
	}

	dirs := manifestDirs(artefacts)

	if !dirs.Has(".") {
		t.Fatal("expected root directory in manifest dirs")
	}

	if !dirs.Has("safe") || !dirs.Has("safe/path") {
		t.Errorf("expected safe path directories, got %v", dirs.Values())
	}

	if dirs.Has("..") || dirs.Has("../escape") || dirs.Has("/absolute") {
		t.Errorf("unexpected unsafe directories in manifest dirs: %v", dirs.Values())
	}
}

func TestIsRel(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "simple path",
			path: "content/posts",
			want: false,
		},
		{
			name: "path with ..",
			path: "content/../etc",
			want: true,
		},
		{
			name: "path with .. at start",
			path: "../etc",
			want: true,
		},
		{
			name: "path with .. at end",
			path: "content/..",
			want: true,
		},
		{
			name: "dot only",
			path: ".",
			want: false,
		},
		{
			name: "empty",
			path: "",
			want: false,
		},
		{
			name: "path with hidden dir",
			path: "content/.git/config",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRel(tt.path)
			if got != tt.want {
				t.Errorf("isRel(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestArtefactBuilder(t *testing.T) {
	builder := func(w io.Writer) error {
		_, err := w.Write([]byte("test content"))
		return err
	}

	var buf strings.Builder
	err := builder(&buf)

	if err != nil {
		t.Errorf("builder() error = %v", err)
	}

	if buf.String() != "test content" {
		t.Errorf("builder() wrote %q, want %q", buf.String(), "test content")
	}
}
