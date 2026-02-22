package workerutil

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWithPanicRecovery(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{name: "NormalExit_ContextCancel", fn: testNormalExitContextCancel},
		{name: "PanicRecovery_SingleRetry", fn: testPanicRecoverySingleRetry},
		{name: "PanicRecovery_MaxRetriesExhausted", fn: testPanicRecoveryMaxRetriesExhausted},
		{name: "ShutdownDuringRecovery", fn: testShutdownDuringRecovery},
		{name: "ShutdownDelayedRetry", fn: testShutdownDelayedRetry},
		{name: "ContextCancelDuringBackoff", fn: testContextCancelDuringBackoff},
		{name: "ExponentialBackoff_Doubling", fn: testExponentialBackoffDoubling},
		{name: "DefaultOptions", fn: testDefaultOptions},
		{name: "NilCallbacks", fn: testNilCallbacks},
		{name: "WaitGroupTracking", fn: testWaitGroupTracking},
		{name: "LastAttemptSkipsBackoff", fn: testLastAttemptSkipsBackoff},
		{name: "PanicValueTypes", fn: testPanicValueTypes},
		{name: "MaxBackoffLessThanInitialBackoff", fn: testMaxBackoffLessThanInitialBackoff},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

// testNormalExitContextCancel verifies that fn exits normally when the context
// is cancelled, and that OnPanic/OnFatal are never called.
func testNormalExitContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	var panicCalled atomic.Int32
	var fatalCalled atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		MaxRetries:     3,
		OnPanic: func(_ string, _ int) {
			panicCalled.Add(1)
		},
		OnFatal: func(_ string, _ int) {
			fatalCalled.Add(1)
		},
	}

	RunWithPanicRecovery(ctx, "test-normal", &wg, func(ctx context.Context) {
		<-ctx.Done()
	}, opts)

	// Give goroutine time to start, then cancel.
	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()

	if panicCalled.Load() != 0 {
		t.Errorf("OnPanic called %d times, want 0", panicCalled.Load())
	}
	if fatalCalled.Load() != 0 {
		t.Errorf("OnFatal called %d times, want 0", fatalCalled.Load())
	}
}

// testPanicRecoverySingleRetry verifies that a single panic triggers recovery
// and OnPanic(attempt=1), then the worker runs successfully on the second attempt.
func testPanicRecoverySingleRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32
	var panicAttempts []int
	var panicMu sync.Mutex
	var fatalCalled atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		MaxRetries:     5,
		OnPanic: func(_ string, attempt int) {
			panicMu.Lock()
			panicAttempts = append(panicAttempts, attempt)
			panicMu.Unlock()
		},
		OnFatal: func(_ string, _ int) {
			fatalCalled.Add(1)
		},
	}

	RunWithPanicRecovery(ctx, "test-single-retry", &wg, func(ctx context.Context) {
		n := callCount.Add(1)
		if n == 1 {
			panic("intentional test panic")
		}
		// Second call: exit normally.
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != 2 {
		t.Errorf("fn called %d times, want 2 (1 panic + 1 normal)", got)
	}

	panicMu.Lock()
	defer panicMu.Unlock()
	if len(panicAttempts) != 1 {
		t.Fatalf("OnPanic called %d times, want 1", len(panicAttempts))
	}
	if panicAttempts[0] != 1 {
		t.Errorf("OnPanic attempt = %d, want 1", panicAttempts[0])
	}
	if fatalCalled.Load() != 0 {
		t.Errorf("OnFatal called %d times, want 0", fatalCalled.Load())
	}
}

// testPanicRecoveryMaxRetriesExhausted verifies that the worker permanently
// stops and calls OnFatal after MaxRetries consecutive panics (#126).
func testPanicRecoveryMaxRetriesExhausted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	const maxRetries = 3
	var callCount atomic.Int32
	var panicCount atomic.Int32
	var fatalCalled atomic.Int32
	var fatalMaxRetries atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
		MaxRetries:     maxRetries,
		OnPanic: func(_ string, _ int) {
			panicCount.Add(1)
		},
		OnFatal: func(_ string, maxR int) {
			fatalCalled.Add(1)
			fatalMaxRetries.Store(int32(maxR))
		},
	}

	RunWithPanicRecovery(ctx, "test-max-retries", &wg, func(_ context.Context) {
		callCount.Add(1)
		panic("always panic")
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != int32(maxRetries) {
		t.Errorf("fn called %d times, want %d", got, maxRetries)
	}
	if got := panicCount.Load(); got != int32(maxRetries) {
		t.Errorf("OnPanic called %d times, want %d", got, maxRetries)
	}
	if fatalCalled.Load() != 1 {
		t.Fatalf("OnFatal called %d times, want 1", fatalCalled.Load())
	}
	if got := fatalMaxRetries.Load(); got != int32(maxRetries) {
		t.Errorf("OnFatal maxRetries = %d, want %d", got, maxRetries)
	}
}

// testShutdownDuringRecovery verifies that when IsShutdown returns true after
// a panic, the worker stops immediately without retrying.
func testShutdownDuringRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32
	var panicCalled atomic.Int32
	var fatalCalled atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		MaxRetries:     5,
		OnPanic: func(_ string, _ int) {
			panicCalled.Add(1)
		},
		OnFatal: func(_ string, _ int) {
			fatalCalled.Add(1)
		},
		IsShutdown: func() bool {
			// After the first panic, report shutdown.
			return callCount.Load() >= 1
		},
	}

	RunWithPanicRecovery(ctx, "test-shutdown", &wg, func(_ context.Context) {
		callCount.Add(1)
		panic("trigger shutdown check")
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != 1 {
		t.Errorf("fn called %d times, want 1 (shutdown should prevent retry)", got)
	}
	// OnPanic should NOT be called because IsShutdown is checked before OnPanic.
	if panicCalled.Load() != 0 {
		t.Errorf("OnPanic called %d times, want 0 (shutdown exits before OnPanic)", panicCalled.Load())
	}
	if fatalCalled.Load() != 0 {
		t.Errorf("OnFatal called %d times, want 0", fatalCalled.Load())
	}
}

// testShutdownDelayedRetry verifies that IsShutdown returning true on the second
// panic (not the first) allows exactly one retry before stopping.
// This covers the delayed shutdown propagation scenario (S-1).
func testShutdownDelayedRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32
	var panicCount atomic.Int32
	var fatalCalled atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		MaxRetries:     5,
		OnPanic: func(_ string, _ int) {
			panicCount.Add(1)
		},
		OnFatal: func(_ string, _ int) {
			fatalCalled.Add(1)
		},
		IsShutdown: func() bool {
			// Shutdown is "propagated" only after the second call: returns
			// false on attempt 1 (allowing one retry), true on attempt 2+.
			return callCount.Load() >= 2
		},
	}

	RunWithPanicRecovery(ctx, "test-shutdown-delayed", &wg, func(_ context.Context) {
		callCount.Add(1)
		panic("trigger delayed shutdown check")
	}, opts)

	wg.Wait()

	// fn should be called exactly twice: attempt 1 (panic, retry) and attempt 2 (panic, shutdown).
	if got := callCount.Load(); got != 2 {
		t.Errorf("fn called %d times, want 2 (1 retry allowed before shutdown)", got)
	}
	// OnPanic is called for attempt 1 only (attempt 2 exits via IsShutdown before OnPanic).
	if got := panicCount.Load(); got != 1 {
		t.Errorf("OnPanic called %d times, want 1 (only first attempt triggers OnPanic)", got)
	}
	// OnFatal must NOT be called: shutdown stops the loop before exhausting MaxRetries.
	if fatalCalled.Load() != 0 {
		t.Errorf("OnFatal called %d times, want 0 (shutdown prevents MaxRetries exhaustion)", fatalCalled.Load())
	}
}

// testContextCancelDuringBackoff verifies that cancelling the context during
// the backoff wait causes the worker to exit promptly.
func testContextCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	var callCount atomic.Int32

	opts := RecoveryOptions{
		// Long backoff so the timer does not fire before we cancel.
		InitialBackoff: 10 * time.Second,
		MaxBackoff:     10 * time.Second,
		MaxRetries:     5,
	}

	RunWithPanicRecovery(ctx, "test-cancel-backoff", &wg, func(_ context.Context) {
		callCount.Add(1)
		panic("trigger backoff")
	}, opts)

	// Wait for the goroutine to enter the backoff wait, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// The goroutine should exit within a reasonable time despite the long backoff.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: goroutine exited promptly.
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit within 2s after context cancel during backoff")
	}

	if got := callCount.Load(); got != 1 {
		t.Errorf("fn called %d times, want 1", got)
	}
}

// testExponentialBackoffDoubling verifies that the backoff delay doubles on
// each retry: 100ms -> 200ms -> 400ms -> ... -> capped at MaxBackoff.
// Uses time.Since measurements with tolerance for CI jitter.
func testExponentialBackoffDoubling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	const maxRetries = 4
	initialBackoff := 100 * time.Millisecond
	maxBackoff := 1000 * time.Millisecond

	var timestamps []time.Time
	var tsMu sync.Mutex

	opts := RecoveryOptions{
		InitialBackoff: initialBackoff,
		MaxBackoff:     maxBackoff,
		MaxRetries:     maxRetries,
	}

	RunWithPanicRecovery(ctx, "test-backoff", &wg, func(_ context.Context) {
		tsMu.Lock()
		timestamps = append(timestamps, time.Now())
		tsMu.Unlock()
		panic("measure backoff")
	}, opts)

	wg.Wait()

	tsMu.Lock()
	defer tsMu.Unlock()

	if len(timestamps) != maxRetries {
		t.Fatalf("got %d timestamps, want %d", len(timestamps), maxRetries)
	}

	// Expected delays between consecutive attempts: 100ms, 200ms, 400ms
	expectedDelays := []time.Duration{
		initialBackoff,
		initialBackoff * 2,
		initialBackoff * 4,
	}

	// Tolerance: +/- 50% to account for CI scheduling jitter and Windows timer
	// resolution (~15.6ms granularity) which can compound across multiple waits.
	for i := 1; i < len(timestamps); i++ {
		actual := timestamps[i].Sub(timestamps[i-1])
		expected := min(expectedDelays[i-1], maxBackoff)

		lowerBound := expected / 2
		upperBound := expected + expected/2

		if actual < lowerBound || actual > upperBound {
			t.Errorf("delay[%d] = %s, want ~%s (tolerance +/-50%%: %s..%s)",
				i-1, actual, expected, lowerBound, upperBound)
		}
	}
}

// testDefaultOptions verifies that a zero-value RecoveryOptions applies
// sensible defaults (InitialBackoff=100ms, MaxBackoff=5s, MaxRetries=10).
func testDefaultOptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32

	// Zero-value opts: all defaults should apply.
	opts := RecoveryOptions{}

	RunWithPanicRecovery(ctx, "test-defaults", &wg, func(_ context.Context) {
		callCount.Add(1)
		// Exit normally on first call.
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != 1 {
		t.Errorf("fn called %d times, want 1 (normal exit)", got)
	}

	// Verify defaults via applyDefaults.
	applied := opts.applyDefaults()
	if applied.InitialBackoff != defaultInitialBackoff {
		t.Errorf("default InitialBackoff = %s, want %s", applied.InitialBackoff, defaultInitialBackoff)
	}
	if applied.MaxBackoff != defaultMaxBackoff {
		t.Errorf("default MaxBackoff = %s, want %s", applied.MaxBackoff, defaultMaxBackoff)
	}
	if applied.MaxRetries != defaultMaxRetries {
		t.Errorf("default MaxRetries = %d, want %d", applied.MaxRetries, defaultMaxRetries)
	}
}

// testNilCallbacks verifies that nil OnPanic, OnFatal, and IsShutdown do not
// cause a panic when the recovery loop invokes them.
func testNilCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32

	// All callbacks nil: must not panic.
	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
		MaxRetries:     2,
		OnPanic:        nil,
		OnFatal:        nil,
		IsShutdown:     nil,
	}

	RunWithPanicRecovery(ctx, "test-nil-callbacks", &wg, func(_ context.Context) {
		callCount.Add(1)
		panic("nil callback safety check")
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != 2 {
		t.Errorf("fn called %d times, want 2 (MaxRetries=2)", got)
	}
}

// testWaitGroupTracking verifies that wg.Wait() completes for all scenarios,
// confirming the goroutine is properly tracked and does not leak.
func testWaitGroupTracking(t *testing.T) {
	scenarios := []struct {
		name     string
		panicFn  func(ctx context.Context)
		cancelFn func(cancel context.CancelFunc)
	}{
		{
			name: "normal exit",
			panicFn: func(_ context.Context) {
				// immediate normal return
			},
			cancelFn: func(_ context.CancelFunc) {},
		},
		{
			name: "context cancel",
			panicFn: func(ctx context.Context) {
				<-ctx.Done()
			},
			cancelFn: func(cancel context.CancelFunc) {
				time.Sleep(10 * time.Millisecond)
				cancel()
			},
		},
		{
			name: "panic then normal",
			panicFn: func() func(ctx context.Context) {
				var count atomic.Int32
				return func(_ context.Context) {
					if count.Add(1) == 1 {
						panic("once")
					}
				}
			}(),
			cancelFn: func(_ context.CancelFunc) {},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup

			opts := RecoveryOptions{
				InitialBackoff: time.Millisecond,
				MaxBackoff:     5 * time.Millisecond,
				MaxRetries:     3,
			}

			RunWithPanicRecovery(ctx, "wg-track-"+sc.name, &wg, sc.panicFn, opts)

			// Trigger cancel if needed.
			go sc.cancelFn(cancel)

			// wg.Wait must complete within a reasonable time.
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Success: no goroutine leak.
			case <-time.After(5 * time.Second):
				t.Fatal("wg.Wait() did not complete within 5s — goroutine leak suspected")
			}
		})
	}
}

// testLastAttemptSkipsBackoff verifies that after the final (MaxRetries-th) panic,
// OnFatal is called WITHOUT waiting for the backoff delay (#I-6).
func testLastAttemptSkipsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	const maxRetries = 3
	// Use a long backoff to make the difference observable.
	const backoff = 500 * time.Millisecond

	var fatalCalled atomic.Int32
	start := time.Now()
	var fatalTime time.Time
	var fatalMu sync.Mutex

	opts := RecoveryOptions{
		InitialBackoff: backoff,
		MaxBackoff:     backoff,
		MaxRetries:     maxRetries,
		OnFatal: func(_ string, _ int) {
			fatalCalled.Add(1)
			fatalMu.Lock()
			fatalTime = time.Now()
			fatalMu.Unlock()
		},
	}

	RunWithPanicRecovery(ctx, "test-last-skip", &wg, func(_ context.Context) {
		panic("always panic")
	}, opts)

	wg.Wait()

	if fatalCalled.Load() != 1 {
		t.Fatalf("OnFatal called %d times, want 1", fatalCalled.Load())
	}

	fatalMu.Lock()
	elapsed := fatalTime.Sub(start)
	fatalMu.Unlock()

	// With maxRetries=3 and backoff=500ms:
	// - Attempt 1 panics, waits 500ms
	// - Attempt 2 panics, waits 500ms
	// - Attempt 3 panics, NO wait (skip), OnFatal called
	// Total: ~1000ms (2 waits, not 3)
	// Allow generous tolerance for CI scheduler.
	const maxExpected = 2*backoff + 200*time.Millisecond
	if elapsed > maxExpected {
		t.Errorf("elapsed = %s, want <= %s: final attempt should skip backoff", elapsed, maxExpected)
	}
}

// testPanicValueTypes verifies that the recovery loop handles all common panic
// value types without itself panicking: string, error, int, struct, and nil.
// S-6: Ensures the slog.Error formatting handles arbitrary panic values.
func testPanicValueTypes(t *testing.T) {
	type customStruct struct {
		Code    int
		Message string
	}

	panicValues := []struct {
		name  string
		value any
	}{
		{name: "string", value: "something went wrong"},
		{name: "error", value: context.DeadlineExceeded},
		{name: "int", value: 42},
		{name: "struct", value: customStruct{Code: 500, Message: "internal"}},
		{name: "nil", value: nil},
	}

	for _, pv := range panicValues {
		t.Run(pv.name, func(t *testing.T) {
			ctx := t.Context()
			var wg sync.WaitGroup

			var callCount atomic.Int32

			opts := RecoveryOptions{
				InitialBackoff: time.Millisecond,
				MaxBackoff:     2 * time.Millisecond,
				MaxRetries:     2,
			}

			RunWithPanicRecovery(ctx, "test-panic-type-"+pv.name, &wg, func(_ context.Context) {
				n := callCount.Add(1)
				if n == 1 && pv.value != nil {
					panic(pv.value)
				}
				// nil panic value: recover() returns nil, so it looks like a normal
				// exit. The worker should just exit normally without retry.
			}, opts)

			wg.Wait()

			if pv.value == nil {
				// nil panic is not detectable via recover(), so fn runs once normally.
				if got := callCount.Load(); got != 1 {
					t.Errorf("fn called %d times, want 1 for nil panic value", got)
				}
			} else {
				// Non-nil panic: first call panics, second call exits normally.
				if got := callCount.Load(); got != 2 {
					t.Errorf("fn called %d times, want 2 for %s panic value", got, pv.name)
				}
			}
		})
	}
}

// testMaxBackoffLessThanInitialBackoff verifies that contradictory configuration
// (MaxBackoff < InitialBackoff) is auto-corrected: MaxBackoff is promoted to
// InitialBackoff so the backoff sequence remains non-decreasing.
// S-7: Validates the applyDefaults correction logic.
func testMaxBackoffLessThanInitialBackoff(t *testing.T) {
	// Direct applyDefaults test.
	opts := RecoveryOptions{
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond, // contradictory: less than initial
		MaxRetries:     3,
	}
	applied := opts.applyDefaults()

	if applied.MaxBackoff != applied.InitialBackoff {
		t.Errorf("applyDefaults: MaxBackoff = %s, want %s (should match InitialBackoff)",
			applied.MaxBackoff, applied.InitialBackoff)
	}

	// End-to-end: verify the corrected config actually works without hanging.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup

	var callCount atomic.Int32

	RunWithPanicRecovery(ctx, "test-backoff-swap", &wg, func(_ context.Context) {
		n := callCount.Add(1)
		if n <= 2 {
			panic("trigger backoff with swapped config")
		}
		// Third call: exit normally.
	}, opts)

	wg.Wait()

	if got := callCount.Load(); got != 3 {
		t.Errorf("fn called %d times, want 3 (2 panics + 1 normal)", got)
	}
}

// TestRunWithPanicRecoveryConcurrent verifies that multiple workers launched
// concurrently all recover from panics independently and complete properly.
// S-22: Ensures the recovery loop is safe under concurrent execution.
func TestRunWithPanicRecoveryConcurrent(t *testing.T) {
	ctx := t.Context()
	var wg sync.WaitGroup

	const workerCount = 10
	var completedWorkers atomic.Int32
	var totalPanics atomic.Int32

	opts := RecoveryOptions{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		MaxRetries:     3,
		OnPanic: func(_ string, _ int) {
			totalPanics.Add(1)
		},
	}

	for i := range workerCount {
		workerName := "concurrent-worker-" + string(rune('A'+i))
		var callCount atomic.Int32
		RunWithPanicRecovery(ctx, workerName, &wg, func(_ context.Context) {
			n := callCount.Add(1)
			if n == 1 {
				panic("first-call panic for " + workerName)
			}
			// Second call: exit normally.
			completedWorkers.Add(1)
		}, opts)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers completed.
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for concurrent workers to complete")
	}

	if got := completedWorkers.Load(); got != workerCount {
		t.Errorf("completed workers = %d, want %d", got, workerCount)
	}
	if got := totalPanics.Load(); got != workerCount {
		t.Errorf("total panics = %d, want %d (one per worker)", got, workerCount)
	}
}

func TestNextBackoff(t *testing.T) {
	tests := []struct {
		name       string
		current    time.Duration
		maxBackoff time.Duration
		want       time.Duration
	}{
		{
			name:       "zero uses default initial",
			current:    0,
			maxBackoff: 5 * time.Second,
			want:       defaultInitialBackoff,
		},
		{
			name:       "negative uses default initial",
			current:    -time.Second,
			maxBackoff: 5 * time.Second,
			want:       defaultInitialBackoff,
		},
		{
			name:       "doubles under cap",
			current:    200 * time.Millisecond,
			maxBackoff: 5 * time.Second,
			want:       400 * time.Millisecond,
		},
		{
			name:       "caps at max",
			current:    5 * time.Second,
			maxBackoff: 5 * time.Second,
			want:       5 * time.Second,
		},
		{
			name:       "caps when doubling exceeds max",
			current:    3 * time.Second,
			maxBackoff: 5 * time.Second,
			want:       5 * time.Second,
		},
		{
			name:       "overflow guard",
			current:    time.Duration(1<<62 - 1),
			maxBackoff: 5 * time.Second,
			want:       5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextBackoff(tt.current, tt.maxBackoff)
			if got != tt.want {
				t.Errorf("nextBackoff(%s, %s) = %s, want %s",
					tt.current, tt.maxBackoff, got, tt.want)
			}
		})
	}
}
