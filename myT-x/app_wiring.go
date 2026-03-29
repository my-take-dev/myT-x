package main

// app_wiring.go is the DI composition root: all buildXxxServiceDeps functions
// live here so that dependency wiring for every App sub-service can be reviewed
// and maintained in a single location.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"myT-x/internal/config"
	"myT-x/internal/devpanel"
	gitpkg "myT-x/internal/git"
	"myT-x/internal/mcp"
	"myT-x/internal/mcpapi"
	"myT-x/internal/orchestrator"
	"myT-x/internal/scheduler"
	"myT-x/internal/session"
	"myT-x/internal/snapshot"
	"myT-x/internal/taskscheduler"
	"myT-x/internal/tmux"
	"myT-x/internal/workerutil"
	"myT-x/internal/worktree"
)

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

// buildSessionServiceDeps constructs the dependency set for the
// session service, wiring app-layer dependencies.
//
// Initialization order contract: sessionService MUST be built before
// snapshotService. These closures capture app by pointer; dependencies
// such as snapshotService and mcpManager must be non-nil at call-time
// (guaranteed because session operations start only after startup() completes).
// If App initialization order changes, verify that all captured fields are
// available when first called.
func buildSessionServiceDeps(app *App) session.Deps {
	return session.Deps{
		Emitter:        newAppRuntimeEventEmitterAdapter(app),
		IsShuttingDown: func() bool { return app.shuttingDown.Load() },
		RequireSessions: func() (*tmux.SessionManager, error) {
			return app.requireSessions()
		},
		RequireRouter: func() (*tmux.CommandRouter, error) {
			return app.requireRouter()
		},
		GetConfigSnapshot: func() config.Config {
			return app.configState.Snapshot()
		},
		RuntimeContext: func() context.Context {
			return app.runtimeContext()
		},
		RequestSnapshot: func(force bool) {
			app.snapshotService.RequestSnapshot(force)
		},
		// NOTE: EmitBackendEvent uses direct method reference (not a closure wrapper)
		// because app.emitBackendEvent has no nil-guard concern — it is a method on
		// *App and is always available. Other fields use closure wrappers for nil
		// guards or parameter adaptation.
		EmitBackendEvent: app.emitBackendEvent,
		McpCleanupSession: func(sessionName string) {
			if app.mcpManager != nil {
				app.mcpManager.CleanupSession(sessionName)
			} else {
				slog.Debug("[DEBUG-SESSION] McpCleanupSession skipped: mcpManager is nil",
					"session", sessionName)
			}
		},
		CleanupStaleSnapshotState: func(activePaneIDs map[string]struct{}) {
			app.snapshotService.CleanupDetachedPaneStates(
				app.snapshotService.DetachStaleOutputBuffers(activePaneIDs),
			)
		},
	}
}

// ---------------------------------------------------------------------------
// Orchestrator
// ---------------------------------------------------------------------------

// buildOrchestratorServiceDeps constructs the dependency set for the
// orchestrator service, wiring App methods and subsystems.
func buildOrchestratorServiceDeps(app *App) orchestrator.Deps {
	return orchestrator.Deps{
		ConfigPath:           func() string { return app.configState.ConfigPath() },
		FindSessionSnapshot:  app.sessionService.FindSessionSnapshotByName,
		GetActiveSessionName: app.sessionService.GetActiveSessionName,
		// CreateSession uses default options (no worktree, no agent-team env).
		CreateSession: func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
			return app.CreateSession(rootPath, sessionName, CreateSessionOptions{})
		},
		CreatePaneInSession: func(sessionName string) (string, error) {
			return app.CreatePaneInSession(sessionName)
		},
		// KillSession destroys the session without deleting worktrees;
		// orchestrator-created sessions never own worktrees.
		KillSession: func(sessionName string) error {
			return app.KillSession(sessionName, false)
		},
		SplitPane: func(paneID string, horizontal bool) (string, error) {
			return app.SplitPane(paneID, horizontal)
		},
		RenamePane: func(paneID, title string) error {
			return app.RenamePane(paneID, title)
		},
		ApplyLayoutPreset: func(sessionName, preset string) error {
			return app.ApplyLayoutPreset(sessionName, preset)
		},
		SendKeys: func(paneID, text string) error {
			router, err := app.requireRouter()
			if err != nil {
				return err
			}
			return app.sendKeys.sendKeysLiteralWithEnter(router, paneID, text)
		},
		SendKeysPaste: func(paneID, text string) error {
			router, err := app.requireRouter()
			if err != nil {
				return err
			}
			return app.sendKeys.sendKeysLiteralPasteWithEnter(router, paneID, text)
		},
		// SleepFn is intentionally left nil; NewService defaults it to time.Sleep.
		// CheckReady validates router readiness before StartTeam begins side effects.
		CheckReady: func() error {
			_, err := app.requireRouter()
			return err
		},
	}
}

// ---------------------------------------------------------------------------
// DevPanel
// ---------------------------------------------------------------------------

// buildDevPanelServiceDeps constructs the dependency set for the
// devpanel service, wiring app-layer dependencies.
func buildDevPanelServiceDeps(app *App) devpanel.Deps {
	return devpanel.Deps{
		ResolveSessionDir: app.sessionService.ResolveSessionDir,
		IsPathWithinBase:  worktree.IsPathWithinBase,
	}
}

// ---------------------------------------------------------------------------
// Worktree
// ---------------------------------------------------------------------------

// buildWorktreeServiceDeps constructs the dependency set for the
// worktree service, wiring app-layer dependencies.
func buildWorktreeServiceDeps(app *App) worktree.Deps {
	return worktree.Deps{
		Emitter:        newAppRuntimeEventEmitterAdapter(app),
		IsShuttingDown: func() bool { return app.shuttingDown.Load() },
		RequireSessions: func() (*tmux.SessionManager, error) {
			return app.requireSessions()
		},
		RequireSessionsAndRouter: func() (*tmux.SessionManager, error) {
			sessions, _, err := app.requireSessionsAndRouter()
			return sessions, err
		},
		GetConfigSnapshot: func() config.Config {
			return app.configState.Snapshot()
		},
		RuntimeContext: func() context.Context {
			return app.runtimeContext()
		},
		FindAvailableSessionName: app.sessionService.FindAvailableSessionName,
		CreateSession: func(sessionDir, sessionName string, enableAgentTeam, useClaudeEnv, usePaneEnv bool) (string, error) {
			return app.sessionService.CreateSessionForDirectory(sessionDir, sessionName, session.CreateSessionOptions{
				EnableAgentTeam: enableAgentTeam,
				UseClaudeEnv:    useClaudeEnv,
				UsePaneEnv:      usePaneEnv,
			})
		},
		ApplySessionEnvFlags:   session.ApplySessionEnvFlags,
		ActivateCreatedSession: app.sessionService.ActivateCreatedSession,
		RollbackCreatedSession: app.sessionService.RollbackCreatedSession,
		StoreRootPath: func(sessionName, rootPath string) error {
			return app.sessionService.StoreRootPath(sessionName, rootPath)
		},
		RequestSnapshot: func(force bool) {
			app.snapshotService.RequestSnapshot(force)
		},
		FindSessionByWorktreePath: app.sessionService.FindSessionByWorktreePath,
		EmitWorktreeCleanupFailure: func(sessionName, wtPath string, err error) {
			app.sessionService.EmitWorktreeCleanupFailure(sessionName, wtPath, err)
		},
		CleanupOrphanedLocalBranch: func(sessionName string, repo *gitpkg.Repository, branchName string) {
			app.sessionService.CleanupOrphanedLocalWorktreeBranch(sessionName, repo, branchName)
		},
		SetupWGAdd:             func(delta int) { app.setupWG.Add(delta) },
		SetupWGDone:            func() { app.setupWG.Done() },
		RecoverBackgroundPanic: recoverBackgroundPanic,
	}
}

// ---------------------------------------------------------------------------
// MCP API
// ---------------------------------------------------------------------------

// buildMCPAPIServiceDeps constructs the dependency set for the mcpapi service,
// wiring app-layer guard functions and configuration.
func buildMCPAPIServiceDeps(app *App) mcpapi.Deps {
	return mcpapi.Deps{
		RequireMCPManager:  func() (*mcp.Manager, error) { return app.requireMCPManager() },
		RequireMCPRegistry: func() (*mcp.Registry, error) { return app.requireMCPRegistry() },
		BridgeCommand:      func() string { return app.mcpBridgeCommand },
	}
}

// ---------------------------------------------------------------------------
// Snapshot
// ---------------------------------------------------------------------------

// buildSnapshotServiceDeps creates the dependency closure bag for the snapshot
// pipeline service.
func buildSnapshotServiceDeps(app *App) snapshot.Deps {
	emitter := newAppRuntimeEventEmitterAdapter(app)
	return snapshot.Deps{
		RuntimeContext: app.runtimeContext,
		Emitter:        emitter,
		SessionsReady:  func() bool { return app.sessions != nil },
		SessionSnapshot: func() []tmux.SessionSnapshot {
			if app.sessions == nil {
				return nil
			}
			return app.sessions.Snapshot()
		},
		TopologyGeneration: func() uint64 {
			if app.sessions == nil {
				return 0
			}
			return app.sessions.TopologyGeneration()
		},
		UpdateActivityByPaneID: func(paneID string) bool {
			if app.sessions == nil {
				return false
			}
			return app.sessions.UpdateActivityByPaneID(paneID)
		},
		DeliverPaneOutput: func(ctx context.Context, paneID string, data []byte) {
			// Prefer WebSocket binary stream for pane data (avoids Wails IPC JSON overhead).
			// Falls back to Wails IPC when no WebSocket client is connected (e.g. during
			// startup before frontend establishes the WebSocket channel).
			//
			// NOTE (TOCTOU): HasActiveConnection() and BroadcastPaneData are not atomic.
			// If the WebSocket closes between this check and BroadcastPaneData(),
			// BroadcastPaneData returns an error and clears the connection; the data
			// for this flush window (<= outputFlushInterval = 16 ms) is lost.
			// This is an accepted design trade-off: the frontend reconnects via
			// paneDataStream's exponential backoff, and any missed terminal output
			// is at most one flush interval worth of data - invisible to users.
			if app.wsHub != nil && app.wsHub.HasActiveConnection() {
				app.wsHub.BroadcastPaneData(paneID, data)
			} else {
				slog.Debug("[output] flushing to frontend via Wails IPC", "paneId", paneID, "flushedLen", len(data))
				app.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(data))
			}
		},
		// PaneState closures: app.paneStates is guaranteed non-nil (initialized in NewApp).
		// The Service also defaults all PaneState closures to no-op in NewService,
		// so nil checks are unnecessary here.
		PaneStateFeedTrimmed: func(paneID string, chunk []byte) {
			app.paneStates.FeedTrimmed(paneID, chunk)
		},
		PaneStateEnsurePane: func(paneID string, width, height int) {
			app.paneStates.EnsurePane(paneID, width, height)
		},
		PaneStateSetActive: func(active map[string]struct{}) {
			app.paneStates.SetActivePanes(active)
		},
		PaneStateRetainPanes: func(alive map[string]struct{}) {
			app.paneStates.RetainPanes(alive)
		},
		PaneStateRemovePane: func(paneID string) {
			app.paneStates.RemovePane(paneID)
		},
		HasPaneStates: func() bool { return app.paneStates != nil },
		LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
			workerutil.RunWithPanicRecovery(ctx, name, &app.bgWG, fn, opts)
		},
		BaseRecoveryOptions: app.defaultRecoveryOptions,
	}
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

// buildSchedulerServiceDeps constructs the dependency set for the
// scheduler service, wiring app-layer dependencies.
func buildSchedulerServiceDeps(app *App) scheduler.Deps {
	return scheduler.Deps{
		Emitter:        newAppRuntimeEventEmitterAdapter(app),
		IsShuttingDown: func() bool { return app.shuttingDown.Load() },
		IsPaneQuiet: func(paneID string) bool {
			return app.snapshotService.IsPaneQuiet(paneID)
		},
		CheckPaneAlive: func(paneID string) error {
			sessions, err := app.requireSessions()
			if err != nil {
				return err
			}
			if !isPaneAlive(sessions, paneID) {
				return fmt.Errorf("pane %s does not exist", paneID)
			}
			return nil
		},
		SendMessage: func(paneID, message string) error {
			router, err := app.requireRouter()
			if err != nil {
				return err
			}
			return app.sendKeys.schedulerSendMessage(router, paneID, message)
		},
		ResolveSessionRootPath: func(sessionName string) (string, error) {
			sessions, err := app.requireSessions()
			if err != nil {
				return "", err
			}
			for _, snap := range sessions.Snapshot() {
				if snap.Name == sessionName {
					if snap.RootPath == "" {
						return "", errors.New("session has no root path")
					}
					return snap.RootPath, nil
				}
			}
			return "", fmt.Errorf("session %s not found", sessionName)
		},
		NewContext: func() (context.Context, context.CancelFunc) {
			parentCtx := app.runtimeContext()
			if parentCtx == nil {
				slog.Warn("[SCHEDULER] NewContext: runtime context nil, falling back to background context")
				parentCtx = context.Background()
			}
			return context.WithCancel(parentCtx)
		},
		LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
			workerutil.RunWithPanicRecovery(ctx, name, &app.bgWG, fn, opts)
		},
		BaseRecoveryOptions: app.defaultRecoveryOptions,
	}
}

// ---------------------------------------------------------------------------
// Task Scheduler
// ---------------------------------------------------------------------------

// buildTaskSchedulerDepsFactory returns a factory that creates a Deps
// bound to a specific session name. Each session gets its own Service
// with closures that reference the fixed sessionName.
func buildTaskSchedulerDepsFactory(app *App) taskscheduler.DepsFactory {
	return func(sessionName string) taskscheduler.Deps {
		return taskscheduler.Deps{
			Emitter:        newAppRuntimeEventEmitterAdapter(app),
			IsShuttingDown: func() bool { return app.shuttingDown.Load() },
			CheckPaneAlive: func(paneID string) error {
				sessions, err := app.requireSessions()
				if err != nil {
					return err
				}
				if !isPaneAlive(sessions, paneID) {
					return fmt.Errorf("pane %s does not exist", paneID)
				}
				return nil
			},
			SendMessagePaste: func(paneID, message string) error {
				router, err := app.requireRouter()
				if err != nil {
					return err
				}
				return app.sendKeys.sendKeysLiteralPasteWithEnter(router, paneID, message)
			},
			ResolveOrchestratorDBPath: func() (string, error) {
				sessions, err := app.requireSessions()
				if err != nil {
					return "", err
				}
				for _, snap := range sessions.Snapshot() {
					if snap.Name == sessionName {
						if snap.RootPath == "" {
							return "", errors.New("session has no root path")
						}
						return filepath.Join(snap.RootPath, ".myT-x", "orchestrator.db"), nil
					}
				}
				return "", fmt.Errorf("session %s not found", sessionName)
			},
			NewContext: func() (context.Context, context.CancelFunc) {
				parentCtx := app.runtimeContext()
				if parentCtx == nil {
					slog.Warn("[DEBUG-TASK-SCHEDULER] NewContext: runtime context nil, falling back to background context")
					parentCtx = context.Background()
				}
				return context.WithCancel(parentCtx)
			},
			LaunchWorker: func(name string, ctx context.Context, fn func(ctx context.Context), opts workerutil.RecoveryOptions) {
				workerutil.RunWithPanicRecovery(ctx, name, &app.bgWG, fn, opts)
			},
			BaseRecoveryOptions: app.defaultRecoveryOptions,
			SendClearCommand: func(paneID, command string) error {
				router, err := app.requireRouter()
				if err != nil {
					return err
				}
				return app.sendKeys.sendKeysLiteralWithEnter(router, paneID, command)
			},
			SessionName: sessionName,
		}
	}
}
