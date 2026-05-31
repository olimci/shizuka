//go:build windows

package console

import (
	"os"

	"golang.org/x/sys/windows"
)

func disableEcho(f *os.File) (func() error, error) {
	return updateConsoleMode(f, func(mode uint32) uint32 {
		return mode &^ windows.ENABLE_ECHO_INPUT
	})
}

func enterCBreak(f *os.File) (func() error, error) {
	return updateConsoleMode(f, func(mode uint32) uint32 {
		return mode &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	})
}

func updateConsoleMode(f *os.File, mutate func(uint32) uint32) (func() error, error) {
	handle := windows.Handle(f.Fd())

	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return nil, err
	}

	newMode := mutate(oldMode)

	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return nil, err
	}

	return func() error {
		return windows.SetConsoleMode(handle, oldMode)
	}, nil
}

func supportsEchoControl() bool {
	return true
}
