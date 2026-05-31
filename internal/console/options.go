package console

import (
	"context"
)

type Options struct {
	// HideCursor hides the cursor until Close restores it.
	HideCursor bool

	// NoEcho disables stdin echo while preserving canonical input mode.
	// This means commands still require Enter. Ignored when CBreak is set.
	NoEcho bool

	// CBreak puts stdin into cbreak mode: no echo and no canonical line
	// buffering, so single keystrokes are delivered immediately without
	// Enter. Signal-generating keys (Ctrl-C, Ctrl-Z) still work.
	CBreak bool

	// CleanupSignals closes the console when the process receives an interrupt
	// or termination signal.
	CleanupSignals bool

	// Context is the parent context for the console lifecycle.
	Context context.Context
}
