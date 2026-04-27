//go:build !darwin

package fileutil

import (
	"io/fs"
	"time"
)

func createdAt(_ fs.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
