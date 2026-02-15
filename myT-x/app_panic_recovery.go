package main

import (
	"log/slog"
	"runtime/debug"
	"time"
)

const (
	initialPanicRestartBackoff = 100 * time.Millisecond
	maxPanicRestartBackoff     = 5 * time.Second
	maxPanicRestartRetries     = 10
)

func recoverBackgroundPanic(worker string, recovered any) bool {
	if recovered != nil {
		slog.Error("[DEBUG-PANIC] background goroutine recovered from panic",
			"worker", worker,
			"panic", recovered,
			"stack", string(debug.Stack()),
		)
		return true
	}
	return false
}

func nextPanicRestartBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return initialPanicRestartBackoff
	}
	if current >= maxPanicRestartBackoff {
		return maxPanicRestartBackoff
	}
	next := current * 2
	if next > maxPanicRestartBackoff || next < current {
		return maxPanicRestartBackoff
	}
	return next
}
