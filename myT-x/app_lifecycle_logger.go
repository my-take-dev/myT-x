package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"myT-x/internal/ipc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// appRuntimeLogger abstracts Wails runtime logging so that startup/shutdown
// code can log through Wails when a runtime context is available and fall
// back to slog when it is not (e.g. during early startup or in tests).
// Contract: implementations MUST fall back to slog when ctx is nil.
type appRuntimeLogger interface {
	Warningf(context.Context, string, ...any)
	Infof(context.Context, string, ...any)
	Errorf(context.Context, string, ...any)
}

type wailsRuntimeLogger struct{}

func formatRuntimeLogMessage(message string, args ...any) string {
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

func (wailsRuntimeLogger) Warningf(ctx context.Context, message string, args ...any) {
	if ctx == nil {
		slog.Warn(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogWarningf(ctx, message, args...)
}

func (wailsRuntimeLogger) Infof(ctx context.Context, message string, args ...any) {
	if ctx == nil {
		slog.Info(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogInfof(ctx, message, args...)
}

func (wailsRuntimeLogger) Errorf(ctx context.Context, message string, args ...any) {
	if ctx == nil {
		slog.Error(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogErrorf(ctx, message, args...)
}

var (
	runtimeEventsEmitFn                  = runtime.EventsEmit
	runtimeLogger       appRuntimeLogger = wailsRuntimeLogger{}
	newPipeServerFn                      = ipc.NewPipeServer
)

// safeStderrWriter returns os.Stderr if it is writable, otherwise io.Discard.
//
// NOTE: In Wails GUI mode on Windows the process may have no attached console,
// which makes os.Stderr an invalid file descriptor. Writing to an invalid
// descriptor can panic or silently fail depending on the Go runtime version.
// A single zero-byte write is used as a probe — this is the cheapest validity
// check that exercises the full kernel write path without producing visible
// output. On failure the error is intentionally discarded and io.Discard is
// returned so that slog initialization always succeeds (non-fatal fallback).
func safeStderrWriter() io.Writer {
	if _, err := os.Stderr.Write([]byte{}); err != nil {
		return io.Discard
	}
	return os.Stderr
}
