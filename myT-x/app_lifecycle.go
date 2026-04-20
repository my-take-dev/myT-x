package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myT-x/internal/apptypes"
	"myT-x/internal/config"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/ipc"
	"myT-x/internal/mcp"
	"myT-x/internal/mcp/lspmcp/lsppkg"
	"myT-x/internal/mcpapi"
	"myT-x/internal/sessionlog"
	"myT-x/internal/tmux"
	"myT-x/internal/wsserver"
)

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

type sessionScopedLifecycleParticipant struct {
	name    string
	cleanup func(sessionName string) error
	rename  func(oldName, newName string) error
}

const expectedSessionScopedLifecycleParticipantCount = 4

func (a *App) emitSessionCleanupDegraded(component, sessionName string, err error) {
	if err == nil {
		return
	}

	a.emitBackendEvent("session:cleanup-degraded", map[string]string{
		"component":    component,
		"session_name": sessionName,
		"message":      err.Error(),
	})
}

func (a *App) cleanupSessionScopedParticipants(sessionName string, participants []sessionScopedLifecycleParticipant) {
	for _, participant := range participants {
		if err := participant.cleanup(sessionName); err != nil {
			slog.Warn("[SESSION] scoped cleanup failed",
				"component", participant.name,
				"session", sessionName,
				"error", err,
			)
			a.emitSessionCleanupDegraded(participant.name, sessionName, err)
		}
	}
}

func (a *App) reconcileSessionRenameRollbackFailure(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	if strings.TrimSpace(a.sessionService.GetActiveSessionName()) == oldName {
		a.sessionService.SetActiveSessionName(newName)
	}

	var reconcileErrs []error
	for _, participant := range a.sessionScopedLifecycleParticipants() {
		if err := participant.rename(oldName, newName); err != nil {
			wrappedErr := fmt.Errorf("%s session rename reconciliation failed: %w", participant.name, err)
			slog.Warn("[SESSION] rename rollback reconciliation failed",
				"component", participant.name,
				"oldSession", oldName,
				"newSession", newName,
				"error", err,
			)
			a.emitSessionCleanupDegraded(participant.name, newName, wrappedErr)
			reconcileErrs = append(reconcileErrs, wrappedErr)
		}
	}

	a.emitBackendEvent("tmux:session-renamed", map[string]any{
		"oldName": oldName,
		"newName": newName,
	})
	return errors.Join(reconcileErrs...)
}

// sessionScopedLifecycleParticipants returns the session-bound subsystems that
// must move together during session cleanup and rename flows.
func (a *App) sessionScopedLifecycleParticipants() []sessionScopedLifecycleParticipant {
	participants := make([]sessionScopedLifecycleParticipant, 0, expectedSessionScopedLifecycleParticipantCount)
	if a.taskSchedulerManager != nil {
		participants = append(participants, sessionScopedLifecycleParticipant{
			name: "task scheduler",
			cleanup: func(sessionName string) error {
				a.taskSchedulerManager.Remove(sessionName)
				return nil
			},
			rename: func(oldName, newName string) error {
				return a.taskSchedulerManager.Rename(oldName, newName)
			},
		})
	}
	if a.singleTaskRunnerManager != nil {
		participants = append(participants, sessionScopedLifecycleParticipant{
			name: "single task runner",
			cleanup: func(sessionName string) error {
				a.singleTaskRunnerManager.Remove(sessionName)
				return nil
			},
			rename: func(oldName, newName string) error {
				return a.singleTaskRunnerManager.Rename(oldName, newName)
			},
		})
	}
	if a.devpanelService != nil {
		participants = append(participants, sessionScopedLifecycleParticipant{
			name:    "devpanel",
			cleanup: a.devpanelService.CleanupSession,
			rename:  a.devpanelService.RenameSession,
		})
	}
	if a.mcpManager != nil {
		participants = append(participants, sessionScopedLifecycleParticipant{
			name:    "mcp",
			cleanup: a.mcpManager.CleanupSession,
			rename:  a.mcpManager.RenameSession,
		})
	}
	return participants
}

// handleSessionRenamed applies follow-up state updates after a tmux session has
// already been renamed. Follow-up steps are transactional: if one subsystem
// fails, previously migrated subsystems are rolled back before the error is
// returned to the caller.
func (a *App) handleSessionRenamed(oldName, newName string) (retErr error) {
	type renameStep struct {
		name     string
		apply    func() error
		rollback func() error
	}

	participants := a.sessionScopedLifecycleParticipants()
	steps := make([]renameStep, 0, len(participants))
	for _, participant := range participants {
		steps = append(steps, renameStep{
			name:  participant.name,
			apply: func() error { return participant.rename(oldName, newName) },
			rollback: func() error {
				return participant.rename(newName, oldName)
			},
		})
	}

	applied := make([]renameStep, 0, len(steps))
	defer func() {
		if retErr == nil {
			return
		}
		for i := len(applied) - 1; i >= 0; i-- {
			step := applied[i]
			if err := step.rollback(); err != nil {
				slog.Warn("[SESSION] follow-up rollback failed",
					"component", step.name,
					"oldSession", oldName,
					"newSession", newName,
					"error", err,
				)
				retErr = errors.Join(retErr, fmt.Errorf("rollback failed for %s: %w", step.name, err))
			}
		}
	}()

	for _, step := range steps {
		if err := step.apply(); err != nil {
			return fmt.Errorf("%s session rename migration failed: %w", step.name, err)
		}
		applied = append(applied, step)
	}
	return nil
}

// Router-driven session lifecycle changes bypass session.Service, so the router
// callbacks must keep active-session state aligned with the renamed/destroyed session.
func (a *App) finalizeSessionDestroyed(sessionName string) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return
	}
	if strings.TrimSpace(a.sessionService.GetActiveSessionName()) == sessionName {
		a.sessionService.SetActive("")
	}
	a.cleanupSessionScopedParticipants(sessionName, a.sessionScopedLifecycleParticipants())
	sessions, err := a.requireSessions()
	if err != nil {
		slog.Warn("[SESSION] stale pane cleanup skipped after destroy",
			"session", sessionName,
			"error", err,
		)
		return
	}
	if a.snapshotService == nil {
		slog.Warn("[SESSION] stale pane cleanup skipped after destroy",
			"session", sessionName,
			"reason", "snapshot service is nil",
		)
		return
	}
	a.snapshotService.CleanupDetachedPaneStates(
		a.snapshotService.DetachStaleOutputBuffers(sessions.ActivePaneIDs()),
	)
}

// Router-driven session lifecycle changes bypass session.Service, so the router
// callbacks must keep active-session state aligned with the renamed/destroyed session.
func (a *App) handleRouterSessionDestroyed(sessionName string) {
	a.finalizeSessionDestroyed(sessionName)
}

// handleRouterSessionRenameRollbackFailed reconciles App state when the router
// could not roll a failed rename back to the original tmux session name.
func (a *App) handleRouterSessionRenameRollbackFailed(oldName, newName string) {
	if err := a.reconcileSessionRenameRollbackFailure(oldName, newName); err != nil {
		slog.Warn("[SESSION] rename rollback failure left degraded participants",
			"oldSession", oldName,
			"newSession", newName,
			"error", err,
		)
	}
}

func (a *App) handleRouterSessionRenamed(oldName, newName string) (retErr error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	activeSessionRenamed := strings.TrimSpace(a.sessionService.GetActiveSessionName()) == oldName
	if activeSessionRenamed {
		// Pre-update the active session name so participant rename hooks observe
		// the new name while handleSessionRenamed migrates session-scoped state.
		// SetActiveSessionName must stay emit-free here. SetActive runs only after
		// migration succeeds so the frontend rename event is emitted exactly once.
		a.sessionService.SetActiveSessionName(newName)
	}
	defer func() {
		if retErr != nil && activeSessionRenamed {
			a.sessionService.SetActiveSessionName(oldName)
		}
	}()

	if err := a.handleSessionRenamed(oldName, newName); err != nil {
		return err
	}
	if activeSessionRenamed {
		a.sessionService.SetActive(newName)
	}
	return nil
}

func (a *App) newRouterOptions(cfg config.Config) tmux.RouterOptions {
	var claudeEnvVars map[string]string
	if cfg.ClaudeEnv != nil {
		claudeEnvVars = cfg.ClaudeEnv.Vars
	}

	return tmux.RouterOptions{
		DefaultShell: cfg.Shell,
		PipeName:     ipc.DefaultPipeName(),
		HostPID:      os.Getpid(),
		PaneEnv:      cfg.PaneEnv,
		ClaudeEnv:    claudeEnvVars,
		OnSessionDestroyed: func(sessionName string) {
			a.handleRouterSessionDestroyed(sessionName)
		},
		OnSessionRenamed: func(oldName, newName string) error {
			return a.handleRouterSessionRenamed(oldName, newName)
		},
		OnSessionRenameRollbackFailed: func(oldName, newName string) {
			a.handleRouterSessionRenameRollbackFailed(oldName, newName)
		},
		ResolveMCPStdio:     a.ResolveMCPStdio,
		ResolveSessionByCwd: a.sessionService.ResolveSessionByCwd,
	}
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
	a.launchDir = workspace
	a.mcpBridgeCommand = mcpapi.ResolveBridgeCommand()
	configPath := config.DefaultPath()
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
	a.initSessionLog(configPath)
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
	a.initInputHistory(configPath)

	cfg, err := config.EnsureFile(configPath)
	if err != nil {
		// Config load/parse failures are non-fatal by product spec.
		// Continue startup with defaults and surface a warning to the user.
		cfg = config.DefaultConfig()
		a.addPendingConfigLoadWarning(
			fmt.Sprintf("Failed to load config file at startup. Running with defaults. Error: %v", err),
		)
		runtimeLogger.Warningf(ctx, "failed to load config from %s: %v", configPath, err)
	}
	a.configState.Initialize(configPath, cfg)

	a.sessions = tmux.NewSessionManager()
	routerOpts := a.newRouterOptions(cfg)
	slog.Debug("[CONFIG] agent model mapping is handled by tmux-shim")
	a.router = tmux.NewCommandRouter(
		a.sessions,
		apptypes.EventEmitterFunc(a.emitBackendEvent),
		routerOpts,
	)
	// MCP registry and manager initialization.
	a.mcpRegistry = mcp.NewRegistry()
	for _, loadErr := range a.mcpRegistry.LoadFromConfig(mcpapi.MCPServerConfigsToDefinitions(cfg.MCPServers)) {
		warnMsg := fmt.Sprintf("Skipped MCP server config entry: %v", loadErr)
		a.addPendingConfigLoadWarning(warnMsg)
		runtimeLogger.Warningf(ctx, "%s", warnMsg)
	}
	// Register built-in LSP extension definitions.
	// Config entries take priority because they are loaded first;
	// Registry.Register rejects duplicate IDs.
	lspDefs := mcpapi.LSPExtensionMetaToDefinitions(lsppkg.AllExtensionMeta())
	for _, loadErr := range a.mcpRegistry.LoadFromConfig(lspDefs) {
		slog.Debug("[DEBUG-MCP] skipped LSP extension registration", "error", loadErr)
	}
	// Register built-in orchestrator MCP definitions.
	orchDefs := orchestratorMCPDefinitions()
	for _, loadErr := range a.mcpRegistry.LoadFromConfig(orchDefs) {
		slog.Debug("[DEBUG-MCP] skipped built-in orchestrator registration (config override or duplicate id)", "error", loadErr)
	}
	// Register built-in single-task-runner MCP definitions.
	strDefs := singleTaskRunnerMCPDefinitions()
	for _, loadErr := range a.mcpRegistry.LoadFromConfig(strDefs) {
		slog.Debug("[DEBUG-MCP] skipped built-in single-task-runner registration (duplicate id — user config takes priority)", "error", loadErr)
	}
	a.mcpManager = mcp.NewManager(mcp.ManagerConfig{
		Registry:                a.mcpRegistry,
		EmitFn:                  a.emitBackendEvent,
		Router:                  a.router,
		ResolveWorkDir:          a.sessionService.ResolveSessionWorkDir,
		SingleTaskRunnerManager: a.singleTaskRunnerManager,
	})

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

	// Prune stale worktree entries left by abnormal exits.
	// Runs before snapshot to keep git state clean from the start.
	a.pruneStaleWorktreesOnStartup(cfg)

	a.configureGlobalHotkey()
	a.snapshotService.StartPaneFeedWorker(ctx)
	a.startIdleMonitor(ctx)
	a.snapshotService.RequestSnapshot(true)
	// NOTE: flushPendingConfigLoadWarnings is intentionally NOT called here.
	// At this point the frontend has not yet registered its EventsOn() handlers,
	// so any emitted warning events would be lost. Instead, warnings are flushed
	// via GetConfigAndFlushWarnings(), which the frontend calls after Wails
	// initialization is complete.
}

// pruneStaleWorktreesOnStartup removes orphaned git worktree entries
// (directories that no longer exist) from the workspace repository.
// Failures are logged but never block startup.
func (a *App) pruneStaleWorktreesOnStartup(cfg config.Config) {
	if !cfg.Worktree.Enabled {
		return
	}
	if !gitpkg.IsGitRepository(a.launchDir) {
		return
	}
	repo, err := gitpkg.Open(a.launchDir)
	if err != nil {
		slog.Warn("[WARN-GIT] startup worktree prune: failed to open repository",
			"path", a.launchDir, "error", err)
		return
	}
	if err := repo.PruneWorktrees(); err != nil {
		slog.Warn("[WARN-GIT] startup worktree prune failed",
			"path", a.launchDir, "error", err)
	}
}

func (a *App) shutdown(_ context.Context) {
	a.shuttingDown.Store(true)
	logCtx := a.runtimeContext()
	// Stop pane feed worker and clear snapshot timer BEFORE bgWG.Wait
	// so the worker goroutine can exit promptly. Shutdown() will call
	// these again internally (idempotent) as part of full pipeline teardown.
	a.snapshotService.StopPaneFeedWorker()
	a.snapshotService.ClearSnapshotRequestTimer()

	if err := a.StopAllSchedulers(); err != nil {
		slog.Warn("[SCHEDULER] stop-all during shutdown failed", "error", err)
	}
	if a.taskSchedulerManager != nil {
		a.taskSchedulerManager.StopAll()
	}
	if a.singleTaskRunnerManager != nil {
		a.singleTaskRunnerManager.StopAll()
	}

	if a.idleCancel != nil {
		a.idleCancel()
		a.idleCancel = nil
	}
	canceledSetupWorkers := a.cancelTrackedSetupWorkers()
	if canceledSetupWorkers > 0 {
		slog.Debug("[DEBUG-GIT] canceled active setup workers during shutdown", "count", canceledSetupWorkers)
	}
	if !waitWithTimeout(a.bgWG.Wait, shutdownWaitTimeout) {
		runtimeLogger.Warningf(logCtx, "timed out waiting for background workers during shutdown")
	}
	if !waitWithTimeout(a.setupWG.Wait, config.SetupScriptCancellationWait) {
		runtimeLogger.Warningf(logCtx, "timed out waiting for setup workers during shutdown")
	}

	// Flush pending input line buffers immediately after workers stop.
	// This minimizes the window between shuttingDown.Store(true) and buffer
	// persistence, preventing entry loss for partially-typed lines.
	a.flushAllLineBuffers()

	// Shutdown the snapshot pipeline: detach output buffers, cleanup pane states,
	// and reset caches/metrics. paneStates.Reset() is called separately because
	// paneStates is shared with non-snapshot code (e.g. app_pane_api.go).
	a.snapshotService.Shutdown()

	if a.paneStates != nil {
		a.paneStates.Reset()
	}
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
	if a.devpanelService != nil {
		if err := a.devpanelService.StopAllWatchers(); err != nil {
			runtimeLogger.Warningf(logCtx, "devpanel watcher stop failed: %v", err)
		}
	}
	if a.mcpManager != nil {
		// Shutdown path: avoid runtime-dependent frontend lifecycle emissions.
		a.mcpManager.CloseWithoutEvent()
	}
	if a.sessions != nil {
		a.sessions.Close()
	}
	a.closeInputHistory()
	a.closeSessionLog()
}
