package workerutil

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

const (
	// defaultInitialBackoff is the starting delay before the first restart attempt
	// after a background worker panic. 100ms is short enough for fast recovery
	// while avoiding tight retry loops that would consume CPU during cascading
	// failures. Doubles on each subsequent attempt up to defaultMaxBackoff.
	defaultInitialBackoff = 100 * time.Millisecond

	// defaultMaxBackoff caps the exponential backoff between restart attempts.
	// 5s balances recovery latency (keeping user-facing workers responsive)
	// against system stability under repeated panics (preventing CPU spin).
	defaultMaxBackoff = 5 * time.Second

	// defaultMaxRetries limits the total restart attempts before permanent stop.
	// At exponential backoff (100ms -> 200ms -> ... -> 5s), 10 retries span
	// approximately 30 seconds total, giving transient issues (e.g. temporary
	// resource exhaustion) time to resolve while bounding resource consumption.
	defaultMaxRetries = 10
)

// RecoveryOptions configures the panic recovery behavior for RunWithPanicRecovery.
// Zero-value fields use sensible defaults: InitialBackoff=100ms, MaxBackoff=5s,
// MaxRetries=10, nil callbacks are safe no-ops.
//
// Zero-value semantics for numeric fields:
//   - A zero value (0 or 0s) means "use default"; applyDefaults() replaces it.
//   - To disable retries entirely, set MaxRetries to 1 (the worker runs once;
//     if it panics, OnFatal is called immediately with no restart).
//   - There is no "infinite retries" mode; MaxRetries must be a positive integer.
type RecoveryOptions struct {
	// InitialBackoff is the starting delay before the first restart attempt.
	// 0 means default (defaultInitialBackoff); applyDefaults() replaces zero/negative
	// values with the default.
	InitialBackoff time.Duration

	// MaxBackoff caps the exponential backoff between restart attempts.
	// 0 means default (defaultMaxBackoff); applyDefaults() replaces zero/negative
	// values with the default.
	MaxBackoff time.Duration

	// MaxRetries limits the total restart attempts before permanent stop.
	// 0 means default (defaultMaxRetries); applyDefaults() replaces zero/negative
	// values with the default. Set to 1 for "no retries" (run once, then OnFatal).
	MaxRetries int

	// OnPanic is called after each panic recovery, before the backoff wait.
	// worker is the worker name, attempt is 1-based. May be nil.
	OnPanic func(worker string, attempt int)

	// OnFatal is called when MaxRetries is exceeded and the worker is permanently
	// stopped. May be nil.
	OnFatal func(worker string, maxRetries int)

	// IsShutdown returns true when the application is shutting down.
	// When true, the recovery loop exits immediately without retrying.
	// This prevents restart attempts during app teardown (e.g. runtimeContext
	// becomes nil). May be nil (treated as always false).
	IsShutdown func() bool
}

// applyDefaults returns a copy of opts with zero-value fields replaced by
// sensible defaults. This avoids mutating the caller's struct.
// Also corrects contradictory configurations (e.g. MaxBackoff < InitialBackoff).
func (opts RecoveryOptions) applyDefaults() RecoveryOptions {
	if opts.InitialBackoff <= 0 {
		slog.Debug("[DEBUG-WORKER] recovery option out of range, using default",
			"field", "InitialBackoff", "value", opts.InitialBackoff, "default", defaultInitialBackoff)
		opts.InitialBackoff = defaultInitialBackoff
	}
	if opts.MaxBackoff <= 0 {
		slog.Debug("[DEBUG-WORKER] recovery option out of range, using default",
			"field", "MaxBackoff", "value", opts.MaxBackoff, "default", defaultMaxBackoff)
		opts.MaxBackoff = defaultMaxBackoff
	}
	if opts.MaxRetries <= 0 {
		slog.Debug("[DEBUG-WORKER] recovery option out of range, using default",
			"field", "MaxRetries", "value", opts.MaxRetries, "default", defaultMaxRetries)
		opts.MaxRetries = defaultMaxRetries
	}

	// S-7: If MaxBackoff < InitialBackoff, the caller likely swapped the values
	// or misconfigured one of them. Correct by promoting MaxBackoff to
	// InitialBackoff so the backoff sequence is always non-decreasing.
	if opts.MaxBackoff < opts.InitialBackoff {
		slog.Warn("[DEBUG-PANIC] MaxBackoff < InitialBackoff is contradictory, using InitialBackoff as MaxBackoff",
			"initialBackoff", opts.InitialBackoff,
			"maxBackoff", opts.MaxBackoff,
		)
		opts.MaxBackoff = opts.InitialBackoff
	}

	return opts
}

// RunWithPanicRecovery launches fn in a new goroutine with automatic panic
// recovery and exponential backoff retry. The goroutine is tracked via wg
// using wg.Go(), which internally manages Add/Done.
//
// The function fn receives a context that is cancelled when the parent context
// is cancelled. fn should select on ctx.Done() to detect cancellation.
//
// Panic recovery logs the stack trace via slog.Error and optionally notifies
// via opts.OnPanic. After opts.MaxRetries consecutive panics, opts.OnFatal is
// called and the goroutine exits permanently.
//
// Thread-safety: safe to call from any goroutine. wg.Go() ensures the
// goroutine is tracked before returning, preventing a race with wg.Wait().
func RunWithPanicRecovery(
	ctx context.Context,
	name string,
	wg *sync.WaitGroup,
	fn func(ctx context.Context),
	opts RecoveryOptions,
) {
	opts = opts.applyDefaults()

	// wg.Go registers the goroutine with the WaitGroup and launches it.
	// This prevents a race where wg.Wait() returns before the new goroutine
	// has started.
	wg.Go(func() {
		runRecoveryLoop(ctx, name, fn, opts)
	})
}

// runRecoveryLoop executes the panic recovery + exponential backoff retry loop.
// Separated from RunWithPanicRecovery for testability and clarity.
func runRecoveryLoop(
	ctx context.Context,
	name string,
	fn func(ctx context.Context),
	opts RecoveryOptions,
) {
	restartDelay := opts.InitialBackoff

	for attempt := 0; attempt < opts.MaxRetries; attempt++ {
		panicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("[DEBUG-PANIC] background goroutine recovered from panic",
						"worker", name,
						"panic", r,
						"stack", string(debug.Stack()),
					)
					panicked = true
				}
			}()
			fn(ctx)
		}()

		// Normal exit (no panic) or context already cancelled: stop immediately.
		if !panicked || ctx.Err() != nil {
			return
		}

		// Shutdown guard: the application is tearing down, do not restart.
		// NOTE: OnPanic is intentionally NOT called when IsShutdown returns true.
		// During shutdown, the runtime context (e.g. Wails app context) may already
		// be nil, so emitting events or accessing app state would cause secondary
		// panics. The panic is still logged above via slog.Error for diagnostics.
		if opts.IsShutdown != nil && opts.IsShutdown() {
			slog.Info("[DEBUG-PANIC] worker shutdown detected, stopping restart",
				"worker", name,
			)
			return
		}

		slog.Warn("[DEBUG-PANIC] restarting worker after panic",
			"worker", name,
			"restartDelay", restartDelay,
			"attempt", attempt+1,
		)

		// Notify caller of the panic (e.g. to emit a frontend event).
		if opts.OnPanic != nil {
			opts.OnPanic(name, attempt+1)
		}

		// Skip backoff wait on the final attempt: there is no next restart,
		// so delaying here only postpones OnFatal notification unnecessarily.
		if attempt == opts.MaxRetries-1 {
			break
		}

		// Wait for backoff duration or context cancellation, whichever comes first.
		// NOTE: Since Go 1.23, Timer.Stop guarantees no stale values are sent on C
		// after Stop returns, so channel draining after Stop is unnecessary.
		restartTimer := time.NewTimer(restartDelay)
		select {
		case <-ctx.Done():
			restartTimer.Stop()
			return
		case <-restartTimer.C:
		}

		restartDelay = nextBackoff(restartDelay, opts.MaxBackoff)
	}

	// All retry attempts exhausted.
	slog.Error("[DEBUG-PANIC] worker exceeded max retries, giving up",
		"worker", name,
		"maxRetries", opts.MaxRetries,
	)

	if opts.OnFatal != nil {
		opts.OnFatal(name, opts.MaxRetries)
	}
}

// nextBackoff doubles the current backoff duration, capping at maxBackoff.
// Guards against integer overflow: if doubling wraps negative or exceeds the
// cap, maxBackoff is returned.
func nextBackoff(current, maxBackoff time.Duration) time.Duration {
	if current <= 0 {
		return defaultInitialBackoff
	}
	if current >= maxBackoff {
		return maxBackoff
	}
	next := current * 2
	// Overflow guard: time.Duration is int64; doubling a large positive value
	// wraps to negative. Cap at maxBackoff in that case.
	if next > maxBackoff || next < current {
		return maxBackoff
	}
	return next
}
