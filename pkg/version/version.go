package version

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is for storing the version of shizuka
const (
	Major = 0
	Minor = 1
	Patch = 0
)

var ErrInvalidVersion = errors.New("invalid version")

type Version struct {
	Major int
	Minor int
	Patch int
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

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func Parse(s string) (Version, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q (expected x.y.z)", ErrInvalidVersion, s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return Version{}, fmt.Errorf("%w: %q (invalid major)", ErrInvalidVersion, s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, fmt.Errorf("%w: %q (invalid minor)", ErrInvalidVersion, s)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, fmt.Errorf("%w: %q (invalid patch)", ErrInvalidVersion, s)
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
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
	return 0
}

func (v Version) Equal(other Version) bool {
	return v.Compare(other) == 0
}

func (v Version) Less(other Version) bool {
	return v.Compare(other) < 0
}
