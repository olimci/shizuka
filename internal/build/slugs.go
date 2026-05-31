package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

var slugPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func generatedSlug(sourcePath, routePath string, used map[string]string) (string, error) {
	for i := range len(used) + 1 {
		source := fmt.Sprintf("%s\x00%s\x00%d", sourcePath, routePath, i)
		sum := sha256.Sum256([]byte(source))
		slug := hex.EncodeToString(sum[:8])
		if _, ok := used[slug]; !ok {
			return slug, nil
		}
	}
	return "", fmt.Errorf("generate slug: exhausted collision retries")
}
