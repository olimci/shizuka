package console

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"

	"golang.org/x/term"
)

const (
	clearScreen     = "\x1b[2J"
	clearScrollback = "\x1b[3J"
	moveHome        = "\x1b[H"
	showCursor      = "\x1b[?25h"
	hideCursor      = "\x1b[?25l"
)

type Console struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex

	In  *os.File
	Out *os.File
	Err *os.File

	InIsTerminal  bool
	OutIsTerminal bool
	ErrIsTerminal bool

	OutAndErrSame bool

	ColorEnabled bool

	EchoDisabled bool
	CursorHidden bool

	restore []func() error
	closed  bool
}

func Open(in, out, errFile *os.File, opts Options) (*Console, error) {
	parent := opts.Context
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)

	c := &Console{
		ctx:    ctx,
		cancel: cancel,
		In:     firstFile(in, os.Stdin),
		Out:    firstFile(out, os.Stdout),
		Err:    firstFile(errFile, os.Stderr),
	}

	c.InIsTerminal = term.IsTerminal(int(c.In.Fd()))
	c.OutIsTerminal = term.IsTerminal(int(c.Out.Fd()))
	c.ErrIsTerminal = term.IsTerminal(int(c.Err.Fd()))

	c.OutAndErrSame = sameFile(c.Out, c.Err)

	c.ColorEnabled = colorEnabled(c.OutIsTerminal, c.ErrIsTerminal)

	if opts.HideCursor && c.OutIsTerminal {
		if _, err := fmt.Fprint(c.Out, hideCursor); err != nil {
			_ = c.Close()
			return nil, err
		}

		c.CursorHidden = true
		c.restore = append(c.restore, func() error {
			_, err := fmt.Fprint(c.Out, showCursor)
			return err
		})
	}

	switch {
	case opts.CBreak && c.InIsTerminal && supportsEchoControl():
		restore, err := enterCBreak(c.In)
		if err != nil {
			_ = c.Close()
			return nil, err
		}

		c.EchoDisabled = true
		c.restore = append(c.restore, restore)

	case opts.NoEcho && c.InIsTerminal && supportsEchoControl():
		restore, err := disableEcho(c.In)
		if err != nil {
			_ = c.Close()
			return nil, err
		}

		c.EchoDisabled = true
		c.restore = append(c.restore, restore)
	}

	if opts.CleanupSignals {
		c.watchCleanupSignals()
	}

	return c, nil
}

func (c *Console) Context() context.Context {
	return c.ctx
}

func (c *Console) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}

	c.closed = true
	restore := append([]func() error(nil), c.restore...)
	cancel := c.cancel
	c.mu.Unlock()

	var errs []error
	for _, r := range slices.Backward(restore) {
		if err := r(); err != nil {
			errs = append(errs, err)
		}
	}
	if cancel != nil {
		cancel()
	}

	return errors.Join(errs...)
}

func (c *Console) ResetView() error {
	if !c.OutIsTerminal {
		return nil
	}

	_, err := fmt.Fprint(c.Out, clearScreen, clearScrollback, moveHome)
	return err
}

func firstFile(f, fallback *os.File) *os.File {
	if f != nil {
		return f
	}

	return fallback
}

func sameFile(a, b *os.File) bool {
	ai, err := a.Stat()
	if err != nil {
		return false
	}

	bi, err := b.Stat()
	if err != nil {
		return false
	}

	return os.SameFile(ai, bi)
}

func colorEnabled(outTTY bool, errTTY bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return outTTY || errTTY
}
