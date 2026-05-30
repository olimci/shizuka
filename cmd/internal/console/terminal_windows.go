//go:build windows

package console

import (
	"os"

	"golang.org/x/sys/windows"
)

func disableEcho(f *os.File) (func() error, error) {
	handle := windows.Handle(f.Fd())

	var oldMode uint32
	if err := windows.GetConsoleMode(handle, &oldMode); err != nil {
		return nil, err
	}

	newMode := oldMode &^ windows.ENABLE_ECHO_INPUT

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
