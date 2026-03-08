//go:build !windows

package main

import (
	"context"
	"errors"
	"io"
	"time"
)

func bridgeMCPStdio(context.Context, string, time.Duration, io.Reader, io.Writer) error {
	return errors.New("mcp stdio mode is supported only on Windows")
}
