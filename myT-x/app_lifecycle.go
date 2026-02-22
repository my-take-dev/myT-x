package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/install"
	"myT-x/internal/ipc"
	"myT-x/internal/sessionlog"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
	"myT-x/internal/wsserver"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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
	cleanupLegacyShimInstallsFn                    = install.CleanupLegacyShimInstalls
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

	// Initialize session error log before other subsystems so that their
	// startup warnings are captured. Install TeeHandler as the default
	// slog logger to intercept Warn/Error level records.
	//
	// IMPORTANT: The base handler must write directly to os.Stderr, NOT use
	// slog.Default().Handler(). The defaultHandler writes through log.Logger,
	// and slog.SetDefault() bridges log.Logger back through the slog handler.
	// Wrapping defaultHandler would create a cycle:
	//   TeeHandler → defaultHandler → log.Logger → handlerWriter → TeeHandler
	// which deadlocks on log.Logger's internal mutex.
	a.initSessionLog()
	baseHandler := slog.NewTextHandler(safeStderrWriter(), nil)
	teeHandler := sessionlog.NewTeeHandler(baseHandler, slog.LevelWarn, func(ts time.Time, level slog.Level, msg string, group string) {
		entry := SessionLogEntry{
			Timestamp: ts.Format("20060102150405"),
			Level:     strings.ToLower(level.String()),
			Message:   msg,
			Source:    group,
		}
		a.writeSessionLogEntry(entry)
	})
	slog.SetDefault(slog.New(teeHandler))

	cfg, err := config.EnsureFile(a.configPath)
	if err != nil {
		// Config load/parse failures are non-fatal by product spec.
		// Continue startup with defaults and surface a warning to the user.
		cfg = config.DefaultConfig()
		a.addPendingConfigLoadWarning(
			fmt.Sprintf("Failed to load config file at startup. Running with defaults. Error: %v", err),
		)
		runtimeLogger.Warningf(ctx, "failed to load config from %s: %v", a.configPath, err)
	}
	a.setConfigSnapshot(cfg)

	a.sessions = tmux.NewSessionManager()
	var claudeEnvVars map[string]string
	if cfg.ClaudeEnv != nil {
		claudeEnvVars = cfg.ClaudeEnv.Vars
	}
	routerOpts := tmux.RouterOptions{
		DefaultShell: cfg.Shell,
		PipeName:     ipc.DefaultPipeName(),
		HostPID:      os.Getpid(),
		PaneEnv:      cfg.PaneEnv,
		ClaudeEnv:    claudeEnvVars,
	}
	slog.Debug("[CONFIG] agent model mapping is handled by tmux-shim")
	a.router = tmux.NewCommandRouter(
		a.sessions,
		tmux.EventEmitterFunc(a.emitBackendEvent),
		routerOpts,
	)
	a.pipeServer = newPipeServerFn(a.router.PipeName(), a.router)
	if err := a.pipeServer.Start(); err != nil {
		runtimeLogger.Errorf(ctx, "pipe server failed: %v", err)
		a.addPendingConfigLoadWarning(
			fmt.Sprintf("Failed to start tmux IPC pipe server at startup. tmux commands may be unavailable. Error: %v", err),
		)
	} else {
		runtimeLogger.Infof(ctx, "pipe server listening: %s", a.pipeServer.PipeName())
	}

	a.ensureShimReady(workspace)

	// WebSocket server for high-throughput pane data streaming.
	// Binds to localhost with OS-assigned port to avoid conflicts.
	// Failure is non-fatal: output falls back to Wails IPC (slower but functional).
	wsPort := cfg.WebSocketPort
	hub := wsserver.NewHub(wsserver.HubOptions{
		Addr: fmt.Sprintf("127.0.0.1:%d", wsPort),
	})
	if err := hub.Start(ctx); err != nil {
		runtimeLogger.Errorf(ctx, "websocket server failed on port %d: %v", wsPort, err)
		hint := fmt.Sprintf(
			"Failed to start WebSocket server on port %d. Terminal output may be slower. "+
				"The port may be in use; try a different websocket_port in config.yaml. Error: %v",
			wsPort, err,
		)
		a.addPendingConfigLoadWarning(hint)
		// hub is not assigned: a.wsHub remains nil, forcing Wails IPC fallback.
	} else {
		runtimeLogger.Infof(ctx, "websocket server listening: %s", hub.URL())
		// NOTE: Theoretical race: the pipe server is already started above and could
		// receive commands before wsHub is assigned here. This is safe in practice
		// because no sessions exist yet at this point, so no pane output can flow
		// through the WebSocket path until after startup completes.
		a.wsHub = hub
	}

	a.configureGlobalHotkey()
	a.startPaneFeedWorker(ctx)
	a.startIdleMonitor(ctx)
	a.requestSnapshot(true)
	// NOTE: flushPendingConfigLoadWarnings is intentionally NOT called here.
	// At this point the frontend has not yet registered its EventsOn() handlers,
	// so any emitted warning events would be lost. Instead, warnings are flushed
	// via GetConfigAndFlushWarnings(), which the frontend calls after Wails
	// initialization is complete.
}

// ensureShimReady synchronizes the tmux shim on every startup and updates
// the current process PATH so child panes can find the shim binary.
// This prevents stale shim binaries when a new version is distributed.
func (a *App) ensureShimReady(workspace string) {
	// Remove legacy shim directories and stale PATH entries before checking
	// installation state. This ensures NeedsShimInstall sees a clean PATH.
	if err := cleanupLegacyShimInstallsFn(); err != nil {
		slog.Warn("[shim] legacy cleanup failed", "error", err)
	}

	needsInstallBefore, preCheckErr := needsShimInstallFn()
	if preCheckErr != nil {
		slog.Warn("[shim] detection failed", "error", preCheckErr)
	}

	result, installErr := ensureShimInstalledFn(workspace)
	if installErr != nil {
		slog.Warn("[shim] startup sync failed", "error", installErr)
		a.addPendingConfigLoadWarning(
			fmt.Sprintf("tmux shim installation failed at startup. Agent Team features may be unavailable. Error: %v", installErr),
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
		slog.Warn("[shim] resolveShimInstallDirFn failed", "error", dirErr)
	}

	// Final check: update shimAvailable based on current state.
	// preCheckErr / postCheckErr naming mirrors the before/after install phases.
	// When postCheckErr != nil the needsInstallAfter value defaults to false
	// (zero value), which intentionally causes SetShimAvailable(false) — the
	// conservative safe default.
	needsInstallAfter, postCheckErr := needsShimInstallFn()
	if postCheckErr != nil {
		slog.Warn("[shim] post-install check failed", "error", postCheckErr)
	}
	if a.router != nil {
		a.router.SetShimAvailable(!needsInstallAfter && postCheckErr == nil)
	}
}

func (a *App) shutdown(_ context.Context) {
	a.shuttingDown.Store(true)
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
	if a.wsHub != nil {
		if err := a.wsHub.Stop(); err != nil {
			runtimeLogger.Warningf(logCtx, "websocket server stop failed: %v", err)
		}
	}
	if a.sessions != nil {
		a.sessions.Close()
	}
	a.closeSessionLog()
}

func (a *App) startIdleMonitor(parent context.Context) {
	sessions, err := a.requireSessions()
	if err != nil {
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
					a.requestSnapshot(false)
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
// Worker-specific overrides (e.g. different MaxRetries) can be set on the returned
// struct after calling this function. (currently unused)
func (a *App) defaultRecoveryOptions() workerutil.RecoveryOptions {
	return workerutil.RecoveryOptions{
		OnPanic: func(worker string, attempt int) {
			if rtCtx := a.runtimeContext(); rtCtx != nil {
				a.emitRuntimeEventWithContext(rtCtx, "tmux:worker-panic", map[string]any{
					"worker":  worker,
					"attempt": attempt,
				})
			}
		},
		OnFatal: func(worker string, maxRetries int) {
			if fatalCtx := a.runtimeContext(); fatalCtx != nil {
				a.emitRuntimeEventWithContext(fatalCtx, "tmux:worker-fatal", map[string]any{
					"worker":     worker,
					"maxRetries": maxRetries,
				})
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

func (a *App) configureGlobalHotkey() {
	cfg := a.getConfigSnapshot()
	// Early return: hotkeys backend not available (e.g. unsupported platform or test env).
	if a.hotkeys == nil {
		slog.Debug("[HOTKEY] hotkey backend unavailable, skipping registration")
		return
	}
	// Early return: quake-mode disabled in config — global hotkey is only for quake toggle.
	if !cfg.QuakeMode {
		return
	}
	logCtx := a.runtimeContext()
	spec := strings.TrimSpace(cfg.GlobalHotkey)
	if spec == "" {
		slog.Debug("[HOTKEY] global hotkey is empty, skipping registration")
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
		slog.Warn("[IPC] bringWindowToFront dropped because runtime context is nil")
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
	// while OS window operations are in progress. Without this guard,
	// rapid hotkey presses could interleave Show/Hide calls, leaving
	// the window in an indeterminate visible/hidden state because the
	// OS window operations (Show, Hide, SetAlwaysOnTop) are not atomic.
	if !a.windowToggling.CompareAndSwap(false, true) {
		slog.Debug("[HOTKEY] toggle already in progress, skipping")
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
