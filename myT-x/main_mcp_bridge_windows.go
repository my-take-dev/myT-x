//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"

	"myT-x/internal/mcp/pipebridge"
)

const (
	mcpBridgeDialAttemptTimeout = 500 * time.Millisecond
	mcpBridgeInitialBackoff     = 200 * time.Millisecond
	mcpBridgeMaxBackoff         = 1 * time.Second
)

// dialPipeFn is a test seam for overriding winio.DialPipe in unit tests.
var dialPipeFn = func(path string, timeout *time.Duration) (net.Conn, error) {
	return winio.DialPipe(path, timeout)
}

func bridgeMCPStdio(ctx context.Context, pipePath string, dialTimeout time.Duration, stdin io.Reader, stdout io.Writer) error {
	pipePath = strings.TrimSpace(pipePath)
	if !strings.HasPrefix(pipePath, `\\.\pipe\`) {
		return fmt.Errorf("invalid pipe path %q", pipePath)
	}

	deadline := time.Now().Add(dialTimeout)
	backoff := mcpBridgeInitialBackoff
	var lastErr error
	attempt := 0

	for {
		attempt++
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		attemptTimeout := min(mcpBridgeDialAttemptTimeout, remaining)

		conn, err := dialPipeFn(pipePath, &attemptTimeout)
		if err == nil {
			if attempt > 1 {
				slog.Debug("[DEBUG-MCP-BRIDGE] pipe connected after retry",
					"pipePath", pipePath,
					"attempts", attempt,
					"elapsed", time.Since(deadline.Add(-dialTimeout)),
				)
			}
			return pipebridge.Bridge(ctx, stdin, stdout, conn)
		}
		lastErr = err

		slog.Debug("[DEBUG-MCP-BRIDGE] dial attempt failed",
			"pipePath", pipePath,
			"attempt", attempt,
			"error", err,
			"remaining", remaining,
		)

		wait := min(backoff, time.Until(deadline))
		if wait <= 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		backoff = min(backoff*2, mcpBridgeMaxBackoff)
	}

	return fmt.Errorf("dial pipe %q failed after %d attempts over %v: %w", pipePath, attempt, dialTimeout, lastErr)
}
