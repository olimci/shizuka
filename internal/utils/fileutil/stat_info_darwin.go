//go:build darwin

package fileutil

import (
	"io/fs"
	"syscall"
	"time"
)

func createdAt(info fs.FileInfo) (time.Time, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(stat.Birthtimespec.Sec), int64(stat.Birthtimespec.Nsec)), true
}
