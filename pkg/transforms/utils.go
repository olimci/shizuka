package transforms

import "github.com/olimci/shizuka/pkg/utils/pathutil"

func firstNonzero[T comparable](values ...T) T {
	zero := *new(T)
	for _, value := range values {
		if value != zero {
			return value
		}
	}
	return zero
}

var (
	URLPathForContentPath = pathutil.URLPathForContentPath
	CleanSlug             = pathutil.CleanSlug
	CleanURLPath          = pathutil.CleanURLPath
)
