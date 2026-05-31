//go:build linux

package console

import (
	"os"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func disableEcho(f *os.File) (func() error, error) {
	return updateTermios(f, func(t *unix.Termios) {
		t.Lflag &^= unix.ECHO
	})
}

func enterCBreak(f *os.File) (func() error, error) {
	return updateTermios(f, func(t *unix.Termios) {
		t.Lflag &^= unix.ECHO | unix.ICANON
		t.Cc[unix.VMIN] = 1
		t.Cc[unix.VTIME] = 0
	})
}

func updateTermios(f *os.File, mutate func(*unix.Termios)) (func() error, error) {
	fd := int(f.Fd())

	old, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}

	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	mutate(t)

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		return nil, err
	}

	return func() error {
		return term.Restore(fd, old)
	}, nil
}

func supportsEchoControl() bool {
	return true
}
