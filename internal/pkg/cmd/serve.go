package cmd

import (
	"context"
	"net/http"
	"time"
)

// GracefulShutdown shutsdown a server, giving open connections up to
// the timeout duration to close before forceably stopping the server.
// If timeout <= 0, the server will wait indefinitely for connections to close.
func GracefulShutdown(s *http.Server, timeout time.Duration) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return s.Shutdown(ctx)
}
