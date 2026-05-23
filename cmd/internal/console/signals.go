package console

import (
	"os"
	"os/signal"
	"syscall"
)

func (c *Console) watchCleanupSignals() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-c.Context().Done():
			_ = c.Close()
		case <-signals:
			_ = c.Close()
		}

		signal.Stop(signals)
	}()
}
