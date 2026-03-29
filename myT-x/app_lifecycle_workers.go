package main

import (
	"context"
	"log/slog"
	"time"

	"myT-x/internal/workerutil"
)

const shutdownWaitTimeout = 10 * time.Second

func (a *App) startIdleMonitor(parent context.Context) {
	sessions, err := a.requireSessions()
	if err != nil {
		slog.Warn("[idle-monitor] cannot start: sessions unavailable", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(parent)
	a.idleCancel = cancel

	workerutil.RunWithPanicRecovery(ctx, "idle-monitor", &a.bgWG, func(ctx context.Context) {
		nextInterval := sessions.RecommendedIdleCheckInterval()
		if nextInterval <= 0 {
			nextInterval = time.Second
		}
		timer := time.NewTimer(nextInterval)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if sessions.CheckIdleState() {
					a.snapshotService.RequestSnapshot(false)
				}
				nextInterval = sessions.RecommendedIdleCheckInterval()
				if nextInterval <= 0 {
					nextInterval = time.Second
				}
				timer.Reset(nextInterval)
			}
		}
	}, a.defaultRecoveryOptions())
}

// defaultRecoveryOptions returns the standard RecoveryOptions for App background
// workers: notifies the frontend on panic/fatal and exits on shutdown detection.
// Worker-specific overrides (e.g. different MaxRetries) can be set on the
// returned struct after calling this function, though no caller currently
// applies overrides.
func (a *App) defaultRecoveryOptions() workerutil.RecoveryOptions {
	return workerutil.RecoveryOptions{
		OnPanic: func(worker string, attempt int) {
			payload := map[string]any{"worker": worker, "attempt": attempt}
			if rtCtx := a.runtimeContext(); rtCtx != nil {
				a.emitRuntimeEventWithContext(rtCtx, "tmux:worker-panic", payload)
			} else {
				slog.Error("[WORKER] panic event dropped: runtime context nil",
					"worker", worker, "attempt", attempt)
			}
		},
		OnFatal: func(worker string, maxRetries int) {
			payload := map[string]any{"worker": worker, "maxRetries": maxRetries}
			if fatalCtx := a.runtimeContext(); fatalCtx != nil {
				a.emitRuntimeEventWithContext(fatalCtx, "tmux:worker-fatal", payload)
			} else {
				slog.Error("[WORKER] fatal event dropped: runtime context nil",
					"worker", worker, "maxRetries", maxRetries)
			}
		},
		IsShutdown: func() bool { return a.shuttingDown.Load() },
	}
}

func waitWithTimeout(waitFn func(), timeout time.Duration) bool {
	// SAFETY: The goroutine spawned below is intentionally not tracked. On timeout,
	// it will leak until waitFn completes or the process exits. This is safe only
	// for process shutdown paths where the OS reclaims all resources shortly after.
	// Do NOT use this helper for non-shutdown contexts; in tests, ensure waitFn
	// always completes to avoid goroutine leaks across test cases.
	done := make(chan struct{})
	go func() {
		waitFn()
		close(done)
	}()

	// Go 1.23+ guarantees Timer.Stop drains the channel, so manual drain is unnecessary.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
