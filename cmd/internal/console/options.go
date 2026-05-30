package console

import (
	"context"
)

type Options struct {
	// HideCursor hides the cursor until Close restores it.
	HideCursor bool

	// NoEcho disables stdin echo while preserving canonical input mode.
	// This means commands still require Enter.
	NoEcho bool

	// CleanupSignals closes the console when the process receives an interrupt
	// or termination signal.
	CleanupSignals bool

	// Context is the parent context for the console lifecycle.
	Context context.Context
}
