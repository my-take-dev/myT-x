//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"

	"myT-x/internal/mcp/pipebridge"
)

func bridgeMCPStdio(ctx context.Context, pipePath string, dialTimeout time.Duration, stdin io.Reader, stdout io.Writer) error {
	pipePath = strings.TrimSpace(pipePath)
	if !strings.HasPrefix(pipePath, `\\.\pipe\`) {
		return fmt.Errorf("invalid pipe path %q", pipePath)
	}
	timeout := dialTimeout
	conn, err := winio.DialPipe(pipePath, &timeout)
	if err != nil {
		return fmt.Errorf("dial pipe %q failed: %w", pipePath, err)
	}
	return pipebridge.Bridge(ctx, stdin, stdout, conn)
}
