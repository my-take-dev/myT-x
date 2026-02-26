package tmux

import (
	"log/slog"
	"runtime/debug"
	"time"
)

const (
	initialRouterPanicRestartBackoff = 100 * time.Millisecond
	maxRouterPanicRestartBackoff     = 5 * time.Second
)

func recoverRouterPanic(worker string, recovered any) bool {
	if recovered != nil {
		slog.Error("[ERROR-PANIC] router goroutine recovered from panic",
			"worker", worker,
			"panic", recovered,
			"stack", string(debug.Stack()),
		)
		return true
	}
	return false
}

func nextRouterPanicRestartBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return initialRouterPanicRestartBackoff
	}
	if current >= maxRouterPanicRestartBackoff {
		return maxRouterPanicRestartBackoff
	}
	next := current * 2
	if next > maxRouterPanicRestartBackoff || next < current {
		return maxRouterPanicRestartBackoff
	}
	return next
}
