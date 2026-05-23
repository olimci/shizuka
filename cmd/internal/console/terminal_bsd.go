//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package console

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func disableEcho(f *os.File) (func() error, error) {
	fd := int(f.Fd())

	old, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}

	t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return nil, err
	}

	t.Lflag &^= unix.ECHO

	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, t); err != nil {
		return nil, err
	}

	return func() error {
		return term.Restore(fd, old)
	}, nil
}

func supportsEchoControl() bool {
	return true
}
