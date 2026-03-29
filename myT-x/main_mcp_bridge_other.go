//go:build !windows

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

func bridgeMCPStdio(_ context.Context, _ string, _ time.Duration, _ io.Reader, _ io.Writer, _ func(string, *time.Duration) (net.Conn, error)) error {
	return errors.New("mcp stdio mode is supported only on Windows")
}
