package version

import "fmt"

const (
	Major = 0
	Minor = 1
	Patch = 0
)

// String returns the version as a semantic version string
func String() string {
	return fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
}
