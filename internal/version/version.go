package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is for storing the version of shizuka
const (
	Major = 1
	Minor = 0
	Patch = 0
)

var ErrInvalidVersion = errors.New("invalid version")
var ErrIncompatibleVersion = errors.New("incompatible version")

type Version struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

func Current() Version {
	return Version{
		Major: Major,
		Minor: Minor,
		Patch: Patch,
	}
}

// String gives you the string representation of the version
func String() string {
	return Current().String()
}

func Banner(repoLink string) string {
	return fmt.Sprintf(`░█▀▀░█░█░▀█▀░▀▀█░█░█░█░█░█▀█ v%s
░▀▀█░█▀█░░█░░▄▀░░█░█░█▀▄░█▀█
░▀▀▀░▀░▀░▀▀▀░▀▀▀░▀▀▀░▀░▀░▀░▀ %s`, String(), repoLink)
}

func (v Version) String() string {
	out := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		out += "-" + v.PreRelease
	}
	if v.Build != "" {
		out += "+" + v.Build
	}
	return out
}

func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return Version{}, fmt.Errorf("%w: empty version", ErrInvalidVersion)
	}

	build := ""
	if before, after, ok := strings.Cut(s, "+"); ok {
		s = before
		build = after
		if !validIdentifiers(build, false) {
			return Version{}, fmt.Errorf("%w: %q (invalid build metadata)", ErrInvalidVersion, s+"+"+build)
		}
	}

	preRelease := ""
	if before, after, ok := strings.Cut(s, "-"); ok {
		s = before
		preRelease = after
		if !validIdentifiers(preRelease, true) {
			return Version{}, fmt.Errorf("%w: %q (invalid prerelease)", ErrInvalidVersion, s+"-"+preRelease)
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q (expected x.y.z)", ErrInvalidVersion, s)
	}

	major, err := parseCorePart(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("%w: %q (invalid major)", ErrInvalidVersion, s)
	}
	minor, err := parseCorePart(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("%w: %q (invalid minor)", ErrInvalidVersion, s)
	}
	patch, err := parseCorePart(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("%w: %q (invalid patch)", ErrInvalidVersion, s)
	}

	return Version{Major: major, Minor: minor, Patch: patch, PreRelease: preRelease, Build: build}, nil
}

func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return comparePrerelease(v.PreRelease, other.PreRelease)
}

func (v Version) Equal(other Version) bool {
	return v.Compare(other) == 0
}

func (v Version) Less(other Version) bool {
	return v.Compare(other) < 0
}

func (v Version) CompatibleWith(runtime Version) bool {
	if v.Major != runtime.Major {
		return false
	}
	if v.Major == 0 {
		return v.Minor == runtime.Minor && v.Compare(runtime) <= 0
	}
	return v.Compare(runtime) <= 0
}

func CheckCompatible(value string) error {
	parsed, err := Parse(value)
	if err != nil {
		return err
	}
	current := Current()
	if !parsed.CompatibleWith(current) {
		return fmt.Errorf("%w: config version %s is not compatible with shizuka %s", ErrIncompatibleVersion, parsed, current)
	}
	return nil
}

func parseCorePart(part string) (int, error) {
	if part == "" {
		return 0, ErrInvalidVersion
	}
	if len(part) > 1 && part[0] == '0' {
		return 0, ErrInvalidVersion
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return 0, ErrInvalidVersion
		}
	}
	return strconv.Atoi(part)
}

func validIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	parts := strings.SplitSeq(value, ".")
	for part := range parts {
		if part == "" {
			return false
		}
		numeric := true
		for _, r := range part {
			if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-' {
				if r < '0' || r > '9' {
					numeric = false
				}
				continue
			}
			return false
		}
		if rejectNumericLeadingZero && numeric && len(part) > 1 && part[0] == '0' {
			return false
		}
	}
	return true
}

func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		cmp := comparePrereleaseIdentifier(aParts[i], bParts[i])
		if cmp != 0 {
			return cmp
		}
	}
	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(a, b string) int {
	aNum, aErr := strconv.Atoi(a)
	bNum, bErr := strconv.Atoi(b)
	aIsNum := aErr == nil
	bIsNum := bErr == nil
	switch {
	case aIsNum && bIsNum:
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
		return 0
	case aIsNum:
		return -1
	case bIsNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}
