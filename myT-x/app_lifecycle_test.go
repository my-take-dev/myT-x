package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"myT-x/internal/install"
	"myT-x/internal/ipc"
	"myT-x/internal/panestate"
	"myT-x/internal/tmux"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn, ensureShimInstalledFn, etc.). Do not use t.Parallel() here.
// Package-level variable replacement makes tests inherently serial; t.Parallel()
// would cause data races between tests that swap the same function variables.

type lifecycleTestLogger struct {
	warnf  func(context.Context, string, ...any)
	infof  func(context.Context, string, ...any)
	errorf func(context.Context, string, ...any)
}

func (l lifecycleTestLogger) Warningf(ctx context.Context, message string, args ...any) {
	if l.warnf != nil {
		l.warnf(ctx, message, args...)
	}
}

func (l lifecycleTestLogger) Infof(ctx context.Context, message string, args ...any) {
	if l.infof != nil {
		l.infof(ctx, message, args...)
	}
}

func (l lifecycleTestLogger) Errorf(ctx context.Context, message string, args ...any) {
	if l.errorf != nil {
		l.errorf(ctx, message, args...)
	}
}

func restoreAllLifecycleHooks() {
	cleanupLegacyShimInstallsFn = install.CleanupLegacyShimInstalls
	needsShimInstallFn = install.NeedsShimInstall
	ensureShimInstalledFn = install.EnsureShimInstalled
	resolveShimInstallDirFn = install.ResolveInstallDir
	ensureProcessPathContainsFn = install.EnsureProcessPathContains
	runtimeLogger = wailsRuntimeLogger{}
	newPipeServerFn = ipc.NewPipeServer
	runtimeWindowIsMinimisedFn = runtime.WindowIsMinimised
	runtimeWindowHideFn = runtime.WindowHide
	runtimeWindowShowFn = runtime.WindowShow
	runtimeWindowUnminimiseFn = runtime.WindowUnminimise
	runtimeWindowSetAlwaysOnTopFn = runtime.WindowSetAlwaysOnTop
}

func newLifecycleTestApp() *App {
	app := NewApp()
	app.router = tmux.NewCommandRouter(nil, nil, tmux.RouterOptions{
		PipeName: ipc.DefaultPipeName(),
	})
	return app
}

func TestEnsureShimReadyCallsLegacyCleanup(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	cleanupCalled := false
	cleanupLegacyShimInstallsFn = func() error {
		cleanupCalled = true
		return nil
	}
	needsShimInstallFn = func() (bool, error) {
		return false, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	if !cleanupCalled {
		t.Fatal("cleanupLegacyShimInstallsFn should have been called")
	}
}

func TestEnsureShimReadyAlwaysRunsStartupSync(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	installCalls := 0
	needsCalls := 0
	events := 0

	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		needsCalls++
		return false, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		installCalls++
		return install.ShimInstallResult{InstalledPath: `C:\Users\test\AppData\Local\myT-x\bin\tmux.exe`}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {
		events++
	}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	if installCalls != 1 {
		t.Fatalf("ensureShimInstalled call count = %d, want 1", installCalls)
	}
	if needsCalls != 2 {
		t.Fatalf("NeedsShimInstall call count = %d, want 2", needsCalls)
	}
	if events != 0 {
		t.Fatalf("runtime event count = %d, want 0", events)
	}
	if !app.router.ShimAvailable() {
		t.Fatal("shim should be available after successful startup sync")
	}
}

func TestEnsureShimReadyEmitsInstallEventWhenPreviouslyMissing(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	needsCalls := 0
	events := 0

	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		needsCalls++
		if needsCalls == 1 {
			return true, nil
		}
		return false, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{InstalledPath: `C:\Users\test\AppData\Local\myT-x\bin\tmux.exe`}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {
		events++
	}

	app := newLifecycleTestApp()
	app.setRuntimeContext(context.Background())
	app.ensureShimReady(`C:\workspace\myT-x`)

	if events != 1 {
		t.Fatalf("runtime event count = %d, want 1", events)
	}
	if !app.router.ShimAvailable() {
		t.Fatal("shim should be available after successful install")
	}
}

func TestEnsureShimReadySkipsPathMutationWhenInstallDirResolutionFails(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	ensurePathCalls := 0
	needsCalls := 0
	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		needsCalls++
		return false, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{InstalledPath: `C:\Users\test\AppData\Local\myT-x\bin\tmux.exe`}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return "", context.DeadlineExceeded
	}
	ensureProcessPathContainsFn = func(string) bool {
		ensurePathCalls++
		return true
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	if ensurePathCalls != 0 {
		t.Fatalf("ensureProcessPathContains call count = %d, want 0", ensurePathCalls)
	}
	if needsCalls != 2 {
		t.Fatalf("NeedsShimInstall call count = %d, want 2", needsCalls)
	}
	if !app.router.ShimAvailable() {
		t.Fatal("shim should remain available when post-check succeeds")
	}
}

func TestEnsureShimReadyMarksShimUnavailableWhenPostCheckFails(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		return true, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{}, context.Canceled
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	if app.router.ShimAvailable() {
		t.Fatal("shim should be unavailable when post-check reports install still needed")
	}
}

func TestEnsureShimReadyMarksShimUnavailableWhenPostCheckErrors(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	needsCalls := 0
	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		needsCalls++
		if needsCalls == 1 {
			return false, nil
		}
		return false, context.DeadlineExceeded
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{InstalledPath: `C:\Users\test\AppData\Local\myT-x\bin\tmux.exe`}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	if needsCalls != 2 {
		t.Fatalf("NeedsShimInstall call count = %d, want 2", needsCalls)
	}
	if app.router.ShimAvailable() {
		t.Fatal("shim should be unavailable when post-check returns an error")
	}
}

func TestEnsureShimReadyAddsStartupWarningWhenInstallFails(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		return true, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{}, context.Canceled
	}
	resolveShimInstallDirFn = func() (string, error) {
		return `C:\Users\test\AppData\Local\myT-x\bin`, nil
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	runtimeEventsEmitFn = func(context.Context, string, ...any) {}

	app := newLifecycleTestApp()
	app.ensureShimReady(`C:\workspace\myT-x`)

	warning := app.consumePendingConfigLoadWarning()
	if !strings.Contains(warning, "tmux shim installation failed at startup") {
		t.Fatalf("startup warning = %q, want shim installation warning", warning)
	}
}

func TestStartupAddsWarningWhenPipeServerStartFails(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)
	origEmit := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

	cleanupLegacyShimInstallsFn = func() error { return nil }
	needsShimInstallFn = func() (bool, error) {
		return false, nil
	}
	ensureShimInstalledFn = func(_ string) (install.ShimInstallResult, error) {
		return install.ShimInstallResult{}, nil
	}
	resolveShimInstallDirFn = func() (string, error) {
		return "", context.DeadlineExceeded
	}
	ensureProcessPathContainsFn = func(string) bool {
		return false
	}
	var emittedWarning string
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		if name != "config:load-failed" || len(data) == 0 {
			return
		}
		payload, ok := data[0].(map[string]string)
		if !ok {
			return
		}
		emittedWarning = payload["message"]
	}
	runtimeLogger = lifecycleTestLogger{}
	newPipeServerFn = func(pipeName string, _ ipc.CommandExecutor) *ipc.PipeServer {
		return ipc.NewPipeServer(pipeName, nil)
	}

	originalSlogHandler := slog.Default()

	app := NewApp()
	app.hotkeys = nil
	app.startup(context.Background())
	t.Cleanup(func() {
		app.shutdown(context.Background())
		slog.SetDefault(originalSlogHandler)
	})

	// startup() no longer calls flushPendingConfigLoadWarnings() directly because
	// the frontend has not yet registered its EventsOn() handlers at that point.
	// Warnings are deferred until GetConfigAndFlushWarnings() is called.
	if emittedWarning != "" {
		t.Fatalf("startup should not emit config:load-failed directly; got %q", emittedWarning)
	}

	// Simulate the frontend calling GetConfigAndFlushWarnings after Wails init.
	_ = app.GetConfigAndFlushWarnings()
	if !strings.Contains(emittedWarning, "Failed to start tmux IPC pipe server at startup.") {
		t.Fatalf("GetConfigAndFlushWarnings warning = %q, want pipe server startup failure warning", emittedWarning)
	}
}

func TestShutdownReleasesInMemoryResources(t *testing.T) {
	stubRuntimeEventsEmit(t)

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.sessions = tmux.NewSessionManager()
	app.paneStates = panestate.NewManager(1024)

	// Prime session and pane state.
	if _, _, err := app.sessions.CreateSession("session-a", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	app.paneStates.EnsurePane("%1", 120, 40)

	// Prime the snapshot pipeline via the service.
	app.snapshotService.RequestSnapshot(true)

	app.snapshotService.StartPaneFeedWorker(context.Background())
	app.startIdleMonitor(context.Background())
	if app.idleCancel == nil {
		t.Fatal("idleCancel should be initialized before shutdown")
	}

	app.shutdown(context.Background())

	if app.idleCancel != nil {
		t.Fatal("idleCancel should be nil after shutdown")
	}
	if app.paneStates.Snapshot("%1") != "" {
		t.Fatal("paneStates should be reset after shutdown")
	}
	if got := len(app.sessions.Snapshot()); got != 0 {
		t.Fatalf("session count = %d, want 0 after shutdown", got)
	}
}

func TestShutdownWaitsForTrackedSetupGoroutines(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setupWG.Add(1)

	done := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("shutdown() returned before setupWG.Done()")
	case <-time.After(100 * time.Millisecond):
	}

	app.setupWG.Done()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown() timed out waiting for setupWG")
	}
}

// TestShutdownCompletesWithoutDeadlock verifies that shutdown() completes within
// a reasonable timeout. The output buffer deadlock test (outputMu ↔ Stop callback)
// is covered in internal/snapshot/service_test.go at the Service level.
func TestShutdownCompletesWithoutDeadlock(t *testing.T) {
	app := NewApp()
	app.setRuntimeContext(context.Background())

	done := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown() timed out; possible deadlock")
	}
}

func TestConfigureGlobalHotkeyLogsWhenManagerUnavailable(t *testing.T) {
	app := NewApp()
	app.hotkeys = nil

	var buf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app.configureGlobalHotkey()

	if !strings.Contains(buf.String(), "[HOTKEY] hotkey backend unavailable, skipping registration") {
		t.Fatalf("expected hotkey-backend-unavailable debug log, output=%q", buf.String())
	}
}

func TestConfigureGlobalHotkeyLogsWhenQuakeModeDisabled(t *testing.T) {
	app := NewApp()
	// hotkeys is non-nil but QuakeMode is false (default).

	var buf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app.configureGlobalHotkey()

	if !strings.Contains(buf.String(), "[HOTKEY] quake-mode disabled, skipping global hotkey registration") {
		t.Fatalf("expected quake-mode-disabled debug log, output=%q", buf.String())
	}
}

func TestDefaultRecoveryOptions(t *testing.T) {
	t.Run("OnPanic emits tmux:worker-panic event", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

		var emittedName string
		var emittedPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			emittedName = name
			if len(data) > 0 {
				if m, ok := data[0].(map[string]any); ok {
					emittedPayload = m
				}
			}
		}

		app := NewApp()
		app.setRuntimeContext(context.Background())
		opts := app.defaultRecoveryOptions()

		opts.OnPanic("test-worker", 3)

		if emittedName != "tmux:worker-panic" {
			t.Fatalf("OnPanic event name = %q, want %q", emittedName, "tmux:worker-panic")
		}
		if emittedPayload["worker"] != "test-worker" {
			t.Fatalf("OnPanic payload worker = %v, want %q", emittedPayload["worker"], "test-worker")
		}
		if emittedPayload["attempt"] != 3 {
			t.Fatalf("OnPanic payload attempt = %v, want 3", emittedPayload["attempt"])
		}
	})

	t.Run("OnPanic falls back to slog.Error when runtimeContext is nil", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

		emitted := false
		runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
			emitted = true
		}

		var logBuf bytes.Buffer
		originalLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})))
		t.Cleanup(func() { slog.SetDefault(originalLogger) })

		app := NewApp()
		// runtimeContext is nil by default.
		opts := app.defaultRecoveryOptions()

		// Must not panic and must not emit via runtime events.
		opts.OnPanic("test-worker", 1)

		if emitted {
			t.Fatal("OnPanic should not emit via runtime events when runtimeContext is nil")
		}
		logOutput := logBuf.String()
		if !strings.Contains(logOutput, "[WORKER] panic event dropped: runtime context nil") {
			t.Fatalf("expected slog.Error fallback log, got %q", logOutput)
		}
		if !strings.Contains(logOutput, "test-worker") {
			t.Fatalf("expected worker name in log, got %q", logOutput)
		}
	})

	t.Run("OnFatal emits tmux:worker-fatal event", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

		var emittedName string
		var emittedPayload map[string]any
		runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
			emittedName = name
			if len(data) > 0 {
				if m, ok := data[0].(map[string]any); ok {
					emittedPayload = m
				}
			}
		}

		app := NewApp()
		app.setRuntimeContext(context.Background())
		opts := app.defaultRecoveryOptions()

		opts.OnFatal("test-worker", 10)

		if emittedName != "tmux:worker-fatal" {
			t.Fatalf("OnFatal event name = %q, want %q", emittedName, "tmux:worker-fatal")
		}
		if emittedPayload["worker"] != "test-worker" {
			t.Fatalf("OnFatal payload worker = %v, want %q", emittedPayload["worker"], "test-worker")
		}
		if emittedPayload["maxRetries"] != 10 {
			t.Fatalf("OnFatal payload maxRetries = %v, want 10", emittedPayload["maxRetries"])
		}
	})

	t.Run("OnFatal falls back to slog.Error when runtimeContext is nil", func(t *testing.T) {
		origEmit := runtimeEventsEmitFn
		t.Cleanup(func() { runtimeEventsEmitFn = origEmit })

		emitted := false
		runtimeEventsEmitFn = func(_ context.Context, _ string, _ ...any) {
			emitted = true
		}

		var logBuf bytes.Buffer
		originalLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError})))
		t.Cleanup(func() { slog.SetDefault(originalLogger) })

		app := NewApp()
		opts := app.defaultRecoveryOptions()

		opts.OnFatal("test-worker", 10)

		if emitted {
			t.Fatal("OnFatal should not emit via runtime events when runtimeContext is nil")
		}
		logOutput := logBuf.String()
		if !strings.Contains(logOutput, "[WORKER] fatal event dropped: runtime context nil") {
			t.Fatalf("expected slog.Error fallback log, got %q", logOutput)
		}
		if !strings.Contains(logOutput, "test-worker") {
			t.Fatalf("expected worker name in log, got %q", logOutput)
		}
	})

	t.Run("IsShutdown returns true when shuttingDown is set", func(t *testing.T) {
		app := NewApp()
		app.shuttingDown.Store(true)
		opts := app.defaultRecoveryOptions()

		if !opts.IsShutdown() {
			t.Fatal("IsShutdown should return true when shuttingDown is set")
		}
	})

	t.Run("IsShutdown returns false when shuttingDown is not set", func(t *testing.T) {
		app := NewApp()
		opts := app.defaultRecoveryOptions()

		if opts.IsShutdown() {
			t.Fatal("IsShutdown should return false when shuttingDown is not set")
		}
	})
}

func TestWaitWithTimeout(t *testing.T) {
	t.Run("returns true when wait completes immediately", func(t *testing.T) {
		if ok := waitWithTimeout(func() {}, 200*time.Millisecond); !ok {
			t.Fatal("waitWithTimeout() = false, want true for immediate wait")
		}
	})

	t.Run("returns false when wait exceeds timeout", func(t *testing.T) {
		block := make(chan struct{})
		if ok := waitWithTimeout(func() { <-block }, 20*time.Millisecond); ok {
			t.Fatal("waitWithTimeout() = true, want false on timeout")
		}
		close(block)
	})

	t.Run("returns false for zero timeout when wait is blocked", func(t *testing.T) {
		block := make(chan struct{})
		if ok := waitWithTimeout(func() { <-block }, 0); ok {
			t.Fatal("waitWithTimeout() = true, want false for zero-timeout blocked wait")
		}
		close(block)
	})
}

func TestToggleQuakeWindowRejectsConcurrentToggle(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setWindowVisible(true)

	runtimeWindowIsMinimisedFn = func(context.Context) bool { return false }

	var hideCalls int32
	hideStarted := make(chan struct{}, 1)
	releaseHide := make(chan struct{})
	runtimeWindowHideFn = func(context.Context) {
		hideCalls++
		hideStarted <- struct{}{}
		<-releaseHide
	}
	runtimeWindowShowFn = func(context.Context) {}
	runtimeWindowUnminimiseFn = func(context.Context) {}
	runtimeWindowSetAlwaysOnTopFn = func(context.Context, bool) {}

	firstDone := make(chan struct{})
	go func() {
		// visible=true, not minimised -> currentlyVisible=true -> hide
		app.toggleQuakeWindow()
		close(firstDone)
	}()
	<-hideStarted

	// Second toggle should be rejected by CAS guard while first is in progress.
	app.toggleQuakeWindow()
	close(releaseHide)
	<-firstDone

	app.windowMu.Lock()
	vis := app.windowVisible
	app.windowMu.Unlock()

	if vis {
		t.Fatal("windowVisible should be false after hide toggle")
	}
	if hideCalls != 1 {
		t.Fatalf("runtimeWindowHide calls = %d, want 1 (concurrent toggle should be rejected)", hideCalls)
	}
}

func TestToggleQuakeWindowShowsHiddenWindow(t *testing.T) {
	t.Cleanup(restoreAllLifecycleHooks)

	app := NewApp()
	app.setRuntimeContext(context.Background())
	app.setWindowVisible(false)

	runtimeWindowIsMinimisedFn = func(context.Context) bool { return false }

	showCalled := false
	runtimeWindowShowFn = func(context.Context) { showCalled = true }
	runtimeWindowHideFn = func(context.Context) { t.Fatal("hide should not be called") }
	runtimeWindowUnminimiseFn = func(context.Context) {}
	runtimeWindowSetAlwaysOnTopFn = func(context.Context, bool) {}

	app.toggleQuakeWindow()

	app.windowMu.Lock()
	vis := app.windowVisible
	app.windowMu.Unlock()

	if !vis {
		t.Fatal("windowVisible should be true after show toggle")
	}
	if !showCalled {
		t.Fatal("runtimeWindowShow should have been called")
	}
}

func TestToggleQuakeWindowSkipsWhenContextNil(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app := NewApp()
	// runtimeContext is nil by default

	// Should return immediately without panic.
	app.toggleQuakeWindow()

	// Verify the CAS guard was properly released.
	if app.windowToggling.Load() {
		t.Fatal("windowToggling should be false after toggle with nil context")
	}

	// Verify the warning log was emitted.
	if !strings.Contains(logBuf.String(), "toggleQuakeWindow dropped because runtime context is nil") {
		t.Fatalf("log output = %q, want toggleQuakeWindow nil-context warning", logBuf.String())
	}
}

func TestBringWindowToFrontSkipsWhenContextNil(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	app := NewApp()
	err := app.bringWindowToFront()
	if !errors.Is(err, errRuntimeContextNil) {
		t.Fatalf("bringWindowToFront() error = %v, want %v", err, errRuntimeContextNil)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "bringWindowToFront dropped because runtime context is nil") {
		t.Fatalf("log output = %q, want bringWindowToFront nil-context warning", logOutput)
	}
}

func TestWailsRuntimeLoggerFallsBackOnNilContext(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	logger := wailsRuntimeLogger{}
	logger.Warningf(nil, "warn %d", 1)
	logger.Infof(nil, "info %d", 2)
	logger.Errorf(nil, "error %d", 3)

	output := logBuf.String()
	if !strings.Contains(output, "warn 1") {
		t.Fatalf("log output = %q, want warning fallback message", output)
	}
	if !strings.Contains(output, "info 2") {
		t.Fatalf("log output = %q, want info fallback message", output)
	}
	if !strings.Contains(output, "error 3") {
		t.Fatalf("log output = %q, want error fallback message", output)
	}
}
