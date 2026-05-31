//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly && !windows

package console

import (
	"errors"
	"os"
)

func disableEcho(_ *os.File) (func() error, error) {
	return nil, errors.New("console: disabling echo is not supported on this platform")
}

func enterCBreak(_ *os.File) (func() error, error) {
	return nil, errors.New("console: cbreak mode is not supported on this platform")
}

func supportsEchoControl() bool {
	return false
}
