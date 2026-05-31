//go:build windows

package fileutil

import (
	"io/fs"
	"syscall"
	"time"
)

func createdAt(info fs.FileInfo) (time.Time, bool) {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, stat.CreationTime.Nanoseconds()), true
}
