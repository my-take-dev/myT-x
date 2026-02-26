package main

import (
	"log/slog"
	"runtime/debug"
)

// recoverBackgroundPanic logs a recovered panic from a background goroutine
// and returns whether a panic actually occurred. The boolean return value
// indicates panic occurrence: true means a panic was recovered (and logged),
// false means recovered was nil (no panic). Callers use this to decide
// whether to restart the worker or exit normally.
//
// Used by simple one-shot goroutines (e.g. worktree setup scripts) that need
// panic logging without full retry-loop semantics. For background workers
// requiring exponential backoff retry, use workerutil.RunWithPanicRecovery.
func recoverBackgroundPanic(worker string, recovered any) bool {
	if recovered != nil {
		slog.Error("[ERROR-PANIC] background goroutine recovered from panic",
			"worker", worker,
			"panic", recovered,
			"stack", string(debug.Stack()),
		)
		return true
	}
	return false
}
