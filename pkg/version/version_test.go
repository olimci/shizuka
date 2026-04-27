package version

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCompareAndBanner(t *testing.T) {
	v, err := Parse(" v1.2.3 ")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := v.String(); got != "1.2.3" {
		t.Fatalf("v.String() = %q, want %q", got, "1.2.3")
	}
	if !v.Less(Version{Major: 1, Minor: 2, Patch: 4}) {
		t.Fatal("Less() = false, want true")
	}
	if !v.Equal(MustParse("1.2.3")) {
		t.Fatal("Equal() = false, want true")
	}

	banner := Banner("https://github.com/olimci/shizuka")
	if !strings.Contains(banner, "v"+String()) || !strings.Contains(banner, "https://github.com/olimci/shizuka") {
		t.Fatalf("Banner() = %q, want version and repo link", banner)
	}
}

func TestParseAndMustParseErrors(t *testing.T) {
	if _, err := Parse("1.2"); !errors.Is(err, ErrInvalidVersion) {
		t.Fatalf("Parse() error = %v, want ErrInvalidVersion", err)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("MustParse() did not panic")
		}
	}()
	_ = MustParse("bad")
}
