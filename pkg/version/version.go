package version

import "fmt"

// Version is for storing the version of shizuka
const (
	Major = 0
	Minor = 1
	Patch = 0
)

// String gives you the string representation of the version
func String() string {
	return fmt.Sprintf("%d.%d.%d", Major, Minor, Patch)
}
