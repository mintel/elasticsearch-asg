package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// WithInterrupt returns a Context that will be canceled if a SIGINT or SIGTERM is received.
func WithInterrupt(ctx context.Context) (context.Context, context.CancelFunc) {
	ctxWithCancel, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalCh)
		select {
		case <-signalCh:
		case <-ctx.Done():
		}
	}()
	return ctxWithCancel, cancel
}
