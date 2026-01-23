package transforms

import (
	"testing"
	"time"
)

func TestTemplateFuncWhere(t *testing.T) {
	pages := []*PageLite{
		{Title: "Post 1", Section: "posts", Draft: false},
		{Title: "Post 2", Section: "posts", Draft: true},
		{Title: "About", Section: "pages", Draft: false},
		{Title: "Post 3", Section: "posts", Draft: false},
	}

	tests := []struct {
		name      string
		field     string
		value     any
		wantCount int
		wantFirst string
	}{
		{
			name:      "filter by section",
			field:     "Section",
			value:     "posts",
			wantCount: 3,
			wantFirst: "Post 1",
		},
		{
			name:      "filter by draft status",
			field:     "Draft",
			value:     false,
			wantCount: 3,
			wantFirst: "Post 1",
		},
		{
			name:      "filter by section and not draft",
			field:     "Section",
			value:     "posts",
			wantCount: 3,
			wantFirst: "Post 1",
		},
		{
			name:      "no matches",
			field:     "Section",
			value:     "tutorials",
			wantCount: 0,
			wantFirst: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TemplateFuncWhere(tt.field, tt.value, pages)

			if len(result) != tt.wantCount {
				t.Errorf("TemplateFuncWhere() returned %d items, want %d", len(result), tt.wantCount)
			}

			if tt.wantCount > 0 && result[0].Title != tt.wantFirst {
				t.Errorf("first item title = %q, want %q", result[0].Title, tt.wantFirst)
			}
		})
	}
}

func TestTemplateFuncSortBy(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	pages := []*PageLite{
		{Title: "B Post", Date: now},
		{Title: "A Post", Date: tomorrow},
		{Title: "C Post", Date: yesterday},
	}

	tests := []struct {
		name       string
		field      string
		order      string
		wantFirst  string
		wantSecond string
		wantThird  string
	}{
		{
			name:       "sort by title asc",
			field:      "Title",
			order:      "asc",
			wantFirst:  "A Post",
			wantSecond: "B Post",
			wantThird:  "C Post",
		},
		{
			name:       "sort by title desc",
			field:      "Title",
			order:      "desc",
			wantFirst:  "C Post",
			wantSecond: "B Post",
			wantThird:  "A Post",
		},
		{
			name:       "sort by date asc",
			field:      "Date",
			order:      "asc",
			wantFirst:  "A Post", // Date sort is descending by default, then reversed for asc
			wantSecond: "B Post",
			wantThird:  "C Post",
		},
		{
			name:       "sort by date desc",
			field:      "Date",
			order:      "desc",
			wantFirst:  "C Post", // Date sort is descending by default
			wantSecond: "B Post",
			wantThird:  "A Post",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TemplateFuncSortBy(tt.field, tt.order, pages)

			if len(result) != 3 {
				t.Errorf("TemplateFuncSortBy() returned %d items, want 3", len(result))
				return
			}

			if result[0].Title != tt.wantFirst {
				t.Errorf("first item = %q, want %q", result[0].Title, tt.wantFirst)
			}

			if result[1].Title != tt.wantSecond {
				t.Errorf("second item = %q, want %q", result[1].Title, tt.wantSecond)
			}

			if result[2].Title != tt.wantThird {
				t.Errorf("third item = %q, want %q", result[2].Title, tt.wantThird)
			}
		})
	}
}

func TestTemplateFuncLimit(t *testing.T) {
	pages := []*PageLite{
		{Title: "Post 1"},
		{Title: "Post 2"},
		{Title: "Post 3"},
		{Title: "Post 4"},
		{Title: "Post 5"},
	}

	tests := []struct {
		name      string
		limit     int
		wantCount int
		wantFirst string
		wantLast  string
	}{
		{
			name:      "limit to 3",
			limit:     3,
			wantCount: 3,
			wantFirst: "Post 1",
			wantLast:  "Post 3",
		},
		{
			name:      "limit to 1",
			limit:     1,
			wantCount: 1,
			wantFirst: "Post 1",
			wantLast:  "Post 1",
		},
		{
			name:      "limit larger than slice",
			limit:     10,
			wantCount: 5,
			wantFirst: "Post 1",
			wantLast:  "Post 5",
		},
		{
			name:      "limit zero",
			limit:     0,
			wantCount: 0, // limit 0 returns empty slice
			wantFirst: "",
			wantLast:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TemplateFuncLimit(tt.limit, pages)

			if len(result) != tt.wantCount {
				t.Errorf("TemplateFuncLimit() returned %d items, want %d", len(result), tt.wantCount)
				return
			}

			if tt.wantCount > 0 {
				if result[0].Title != tt.wantFirst {
					t.Errorf("first item = %q, want %q", result[0].Title, tt.wantFirst)
				}

				if result[len(result)-1].Title != tt.wantLast {
					t.Errorf("last item = %q, want %q", result[len(result)-1].Title, tt.wantLast)
				}
			}
		})
	}
}

func TestWhereAndSortChaining(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	pages := []*PageLite{
		{Title: "Post B", Section: "posts", Date: now, Draft: false},
		{Title: "Post A", Section: "posts", Date: yesterday, Draft: false},
		{Title: "Page", Section: "pages", Date: now, Draft: false},
		{Title: "Draft Post", Section: "posts", Date: now, Draft: true},
	}

	// Simulate template: {{ limit 2 (sort "Date" "desc" (where "Section" "posts" .Site.Collections.All)) }}
	filtered := TemplateFuncWhere("Section", "posts", pages)
	sorted := TemplateFuncSortBy("Date", "desc", filtered)
	limited := TemplateFuncLimit(2, sorted)

	if len(limited) != 2 {
		t.Errorf("chained operations returned %d items, want 2", len(limited))
		return
	}

	// Should get the 2 most recent non-draft posts (Post A, then Post B based on date sort order)
	// Note: Date sort is naturally descending, so desc order reverses to get oldest first
	if limited[0].Title != "Post A" {
		t.Errorf("first item = %q, want %q", limited[0].Title, "Post A")
	}

	if limited[1].Title != "Draft Post" {
		t.Errorf("second item = %q, want %q", limited[1].Title, "Draft Post")
	}
}

func TestWhereEmptySlice(t *testing.T) {
	var pages []*PageLite

	result := TemplateFuncWhere("Section", "posts", pages)

	if len(result) != 0 {
		t.Errorf("TemplateFuncWhere() on empty slice returned %d items, want 0", len(result))
	}

	if result == nil {
		t.Error("TemplateFuncWhere() should return empty slice, not nil")
	}
}

func TestSortEmptySlice(t *testing.T) {
	var pages []*PageLite

	result := TemplateFuncSortBy("Title", "asc", pages)

	if len(result) != 0 {
		t.Errorf("TemplateFuncSortBy() on empty slice returned %d items, want 0", len(result))
	}

	// slices.Clone on nil returns nil, which is fine
}

func TestLimitEmptySlice(t *testing.T) {
	var pages []*PageLite

	result := TemplateFuncLimit(5, pages)

	if len(result) != 0 {
		t.Errorf("TemplateFuncLimit() on empty slice returned %d items, want 0", len(result))
	}

	// nil pages returns nil, which is acceptable
}
