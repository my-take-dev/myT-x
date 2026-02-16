package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/install"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type appRuntimeLogger interface {
	Warningf(context.Context, string, ...interface{})
	Infof(context.Context, string, ...interface{})
	Errorf(context.Context, string, ...interface{})
}

type wailsRuntimeLogger struct{}

func formatRuntimeLogMessage(message string, args ...interface{}) string {
	if len(args) == 0 {
		return message
	}
	return fmt.Sprintf(message, args...)
}

func (wailsRuntimeLogger) Warningf(ctx context.Context, message string, args ...interface{}) {
	if ctx == nil {
		slog.Warn(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogWarningf(ctx, message, args...)
}

func (wailsRuntimeLogger) Infof(ctx context.Context, message string, args ...interface{}) {
	if ctx == nil {
		slog.Info(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogInfof(ctx, message, args...)
}

func (wailsRuntimeLogger) Errorf(ctx context.Context, message string, args ...interface{}) {
	if ctx == nil {
		slog.Error(formatRuntimeLogMessage(message, args...))
		return
	}
	runtime.LogErrorf(ctx, message, args...)
}

var (
	needsShimInstallFn                             = install.NeedsShimInstall
	ensureShimInstalledFn                          = install.EnsureShimInstalled
	resolveShimInstallDirFn                        = install.ResolveInstallDir
	ensureProcessPathContainsFn                    = install.EnsureProcessPathContains
	runtimeEventsEmitFn                            = runtime.EventsEmit
	runtimeLogger                 appRuntimeLogger = wailsRuntimeLogger{}
	newPipeServerFn                                = ipc.NewPipeServer
	runtimeWindowIsMinimisedFn                     = runtime.WindowIsMinimised
	runtimeWindowHideFn                            = runtime.WindowHide
	runtimeWindowShowFn                            = runtime.WindowShow
	runtimeWindowUnminimiseFn                      = runtime.WindowUnminimise
	runtimeWindowSetAlwaysOnTopFn                  = runtime.WindowSetAlwaysOnTop
)

const shutdownWaitTimeout = 10 * time.Second

func (a *App) addPendingConfigLoadWarning(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	a.startupWarnMu.Lock()
	a.configLoadWarnings = append(a.configLoadWarnings, trimmed)
	a.startupWarnMu.Unlock()
}

func (a *App) consumePendingConfigLoadWarning() string {
	a.startupWarnMu.Lock()
	defer a.startupWarnMu.Unlock()
	if len(a.configLoadWarnings) == 0 {
		return ""
	}
	message := strings.Join(a.configLoadWarnings, "\n")
	a.configLoadWarnings = nil
	return message
}

func (a *App) startup(ctx context.Context) {
	setConsoleUTF8()

	a.setRuntimeContext(ctx)
	a.setWindowVisible(true)

	workspace, err := os.Getwd()
	if err != nil {
		if exePath, exeErr := os.Executable(); exeErr == nil {
			workspace = filepath.Dir(exePath)
		} else {
			workspace = "."
		}
		runtimeLogger.Warningf(ctx, "failed to resolve working directory: %v", err)
	}
	a.workspace = workspace
	a.configPath = config.DefaultPath()
	for _, message := range config.ConsumeDefaultPathWarnings() {
		a.addPendingConfigLoadWarning(message)
	}

	cfg, err := config.EnsureFile(a.configPath)
	if err != nil {
		// Config load/parse failures are non-fatal by product spec.
		// Continue startup with defaults and surface a warning to the user.
		cfg = config.DefaultConfig()
		a.addPendingConfigLoadWarning(
			"Failed to load config file at startup. Running with defaults. Error: " + err.Error(),
		)
		runtimeLogger.Warningf(ctx, "failed to load config from %s: %v", a.configPath, err)
	}
	a.setConfigSnapshot(cfg)

	a.sessions = tmux.NewSessionManager()
	routerOpts := tmux.RouterOptions{
		DefaultShell: cfg.Shell,
		PipeName:     ipc.DefaultPipeName(),
		HostPID:      os.Getpid(),
		PaneEnv:      cfg.PaneEnv,
	}
	slog.Debug("[DEBUG-CONFIG] agent model mapping is handled by tmux-shim")
	a.router = tmux.NewCommandRouter(
		a.sessions,
		tmux.EventEmitterFunc(a.emitBackendEvent),
		routerOpts,
	)
	a.pipeServer = newPipeServerFn(a.router.PipeName(), a.router)
	if err := a.pipeServer.Start(); err != nil {
		runtimeLogger.Errorf(ctx, "pipe server failed: %v", err)
		a.addPendingConfigLoadWarning(
			"Failed to start tmux IPC pipe server at startup. tmux commands may be unavailable. Error: " + err.Error(),
		)
	} else {
		runtimeLogger.Infof(ctx, "pipe server listening: %s", a.pipeServer.PipeName())
	}

	a.ensureShimReady(workspace)

	a.configureGlobalHotkey()
	a.startPaneFeedWorker(ctx)
	a.startIdleMonitor(ctx)
	a.requestSnapshot(true)
	a.flushPendingConfigLoadWarnings()
}

// ensureShimReady synchronizes the tmux shim on every startup and updates
// the current process PATH so child panes can find the shim binary.
// This prevents stale shim binaries when a new version is distributed.
func (a *App) ensureShimReady(workspace string) {
	needsInstallBefore, err := needsShimInstallFn()
	if err != nil {
		slog.Warn("[shim] detection failed", "error", err)
	}

	result, installErr := ensureShimInstalledFn(workspace)
	if installErr != nil {
		slog.Warn("[shim] startup sync failed", "error", installErr)
		a.addPendingConfigLoadWarning(
			"tmux shim installation failed at startup. Agent Team features may be unavailable. Error: " + installErr.Error(),
		)
	} else {
		slog.Info("[shim] synchronized", "path", result.InstalledPath)
		// Preserve existing event behavior for first-time install scenarios.
		if eventCtx := a.runtimeContext(); needsInstallBefore && eventCtx != nil {
			runtimeEventsEmitFn(eventCtx, "tmux:shim-installed", result)
		}
	}

	// Ensure the shim directory is in the current process PATH so that
	// child processes (panes) inherit it and can find the tmux binary.
	shimDir, dirErr := resolveShimInstallDirFn()
	if dirErr == nil {
		if ensureProcessPathContainsFn(shimDir) {
			slog.Info("[shim] process PATH updated", "shimDir", shimDir)
		}
	} else {
		slog.Warn("[DEBUG-SHIM] resolveShimInstallDirFn failed", "error", dirErr)
	}

	// Final check: update shimAvailable based on current state.
	needsInstallAfter, err := needsShimInstallFn()
	if err != nil {
		slog.Warn("[shim] post-install check failed", "error", err)
	}
	if a.router != nil {
		a.router.SetShimAvailable(!needsInstallAfter && err == nil)
	}
}

func (a *App) shutdown(_ context.Context) {
	logCtx := a.runtimeContext()
	a.stopPaneFeedWorker()
	a.clearSnapshotRequestTimer()
	a.snapshotRequestMu.Lock()
	a.snapshotRequestGeneration = 0
	a.snapshotRequestDispatched = 0
	a.snapshotRequestMu.Unlock()

	if a.idleCancel != nil {
		a.idleCancel()
		a.idleCancel = nil
	}
	if !waitWithTimeout(a.bgWG.Wait, shutdownWaitTimeout) {
		runtimeLogger.Warningf(logCtx, "timed out waiting for background workers during shutdown")
	}
	if !waitWithTimeout(a.setupWG.Wait, shutdownWaitTimeout) {
		runtimeLogger.Warningf(logCtx, "timed out waiting for setup workers during shutdown")
	}

	a.cleanupDetachedPaneStates(a.detachAllOutputBuffers())

	if a.paneStates != nil {
		a.paneStates.Reset()
	}
	a.snapshotMu.Lock()
	a.snapshotCache = map[string]tmux.SessionSnapshot{}
	a.snapshotPrimed = false
	a.snapshotLastTopology = 0
	a.snapshotMu.Unlock()
	a.snapshotMetricsMu.Lock()
	a.snapshotStats = snapshotMetrics{}
	a.snapshotMetricsMu.Unlock()
	if a.hotkeys != nil {
		if err := a.hotkeys.Stop(); err != nil {
			runtimeLogger.Warningf(logCtx, "hotkeys stop failed: %v", err)
		}
	}

	if a.pipeServer != nil {
		if err := a.pipeServer.Stop(); err != nil {
			runtimeLogger.Warningf(logCtx, "pipe server stop failed: %v", err)
		}
	}
	if a.sessions != nil {
		a.sessions.Close()
	}
}

func (a *App) startIdleMonitor(parent context.Context) {
	sessions, err := a.requireSessions()
	if err != nil {
		return
	}

	ctx, cancel := context.WithCancel(parent)
	a.idleCancel = cancel

	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		restartDelay := initialPanicRestartBackoff
		for attempt := 0; attempt < maxPanicRestartRetries; attempt++ {
			panicked := false
			func() {
				defer func() {
					if recoverBackgroundPanic("idle-monitor", recover()) {
						panicked = true
					}
				}()

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
							a.requestSnapshot(false)
						}
						nextInterval = sessions.RecommendedIdleCheckInterval()
						if nextInterval <= 0 {
							nextInterval = time.Second
						}
						timer.Reset(nextInterval)
					}
				}
			}()
			if !panicked || ctx.Err() != nil {
				return
			}
			// Shutdown guard: runtimeContext becomes nil during app teardown.
			if a.runtimeContext() == nil {
				slog.Info("[DEBUG-PANIC] idle-monitor: runtimeContext nil, stopping restart")
				return
			}
			slog.Warn("[DEBUG-PANIC] restarting worker after panic",
				"worker", "idle-monitor",
				"restartDelay", restartDelay,
				"attempt", attempt+1,
			)
			a.emitRuntimeEventWithContext(a.runtimeContext(), "tmux:worker-panic", map[string]any{
				"worker": "idle-monitor",
			})
			restartTimer := time.NewTimer(restartDelay)
			select {
			case <-ctx.Done():
				if !restartTimer.Stop() {
					<-restartTimer.C
				}
				return
			case <-restartTimer.C:
			}
			restartDelay = nextPanicRestartBackoff(restartDelay)
		}
		slog.Error("[DEBUG-PANIC] idle-monitor exceeded max retries, giving up",
			"maxRetries", maxPanicRestartRetries)
	}()
}

func waitWithTimeout(waitFn func(), timeout time.Duration) bool {
	// Best effort timeout guard for shutdown paths. The waiting goroutine may
	// outlive timeout when waitFn blocks indefinitely, but this function is only
	// used during process shutdown where eventual completion is expected.
	done := make(chan struct{})
	go func() {
		waitFn()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func (a *App) configureGlobalHotkey() {
	cfg := a.getConfigSnapshot()
	if a.hotkeys == nil {
		slog.Debug("[DEBUG-hotkey] no hotkeys configured, skipping")
		return
	}
	if !cfg.QuakeMode {
		return
	}
	logCtx := a.runtimeContext()
	spec := strings.TrimSpace(cfg.GlobalHotkey)
	if spec == "" {
		slog.Debug("[DEBUG-hotkey] no hotkeys configured, skipping")
		return
	}

	if err := a.hotkeys.Start(spec, a.toggleQuakeWindow); err != nil {
		runtimeLogger.Warningf(logCtx, "global hotkey registration failed: %v", err)
		return
	}
	runtimeLogger.Infof(logCtx, "global hotkey registered: %s", a.hotkeys.ActiveBinding())
}

// bringWindowToFront shows and raises the application window.
// Used when a second instance signals the first to activate.
func (a *App) bringWindowToFront() {
	ctx := a.runtimeContext()
	if ctx == nil {
		slog.Warn("[DEBUG-IPC] bringWindowToFront dropped because runtime context is nil")
		return
	}
	a.raiseWindow(ctx)
	a.setWindowVisible(true)
}

func (a *App) raiseWindow(ctx context.Context) {
	runtimeWindowShowFn(ctx)
	runtimeWindowUnminimiseFn(ctx)
	runtimeWindowSetAlwaysOnTopFn(ctx, true)
	runtimeWindowSetAlwaysOnTopFn(ctx, false)
}

func (a *App) setWindowVisible(visible bool) {
	a.windowMu.Lock()
	a.windowVisible = visible
	a.windowMu.Unlock()
}

func (a *App) toggleQuakeWindow() {
	// CAS guard prevents double-toggle when a second hotkey fires
	// while OS window operations are in progress.
	if !a.windowToggling.CompareAndSwap(false, true) {
		slog.Debug("[DEBUG-hotkey] toggle already in progress, skipping")
		return
	}
	defer a.windowToggling.Store(false)

	ctx := a.runtimeContext()
	if ctx == nil {
		return
	}

	// Read OS window state outside lock (#78: no Wails runtime API inside mutex).
	isMinimised := runtimeWindowIsMinimisedFn(ctx)

	// Determine action under lock.
	a.windowMu.Lock()
	currentlyVisible := a.windowVisible && !isMinimised
	a.windowMu.Unlock()

	// Perform OS window operations outside lock.
	if currentlyVisible {
		runtimeWindowHideFn(ctx)
	} else {
		a.raiseWindow(ctx)
	}

	a.setWindowVisible(!currentlyVisible)
}
