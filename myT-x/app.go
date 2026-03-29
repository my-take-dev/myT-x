package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

	"myT-x/internal/config"
	"myT-x/internal/devpanel"
	"myT-x/internal/hotkeys"
	"myT-x/internal/inputhistory"
	"myT-x/internal/ipc"
	"myT-x/internal/mcp"
	"myT-x/internal/mcpapi"
	"myT-x/internal/orchestrator"
	"myT-x/internal/panestate"
	"myT-x/internal/scheduler"
	"myT-x/internal/session"
	"myT-x/internal/sessionlog"
	"myT-x/internal/snapshot"
	"myT-x/internal/taskscheduler"
	"myT-x/internal/tmux"
	"myT-x/internal/worktree"
	"myT-x/internal/wsserver"
)

// App is the Wails-bound application service.
type App struct {
	// Runtime context lifecycle.
	ctx   context.Context
	ctxMu sync.RWMutex

	// configState owns the in-memory config snapshot, serialized persistence,
	// and monotonic event versioning. Initialized in NewApp(); config path and
	// initial snapshot are set during startup via configState.Initialize().
	// See config.StateService for lock ordering.
	configState *config.StateService

	// Nested lock ordering (one-way only):
	//   paneEnvUpdateMu -> tmux.CommandRouter.paneEnvMu (via UpdatePaneEnv)
	//   claudeEnvUpdateMu -> tmux.CommandRouter.claudeEnvMu (via UpdateClaudeEnv)
	//
	// Independent locks: do not assume ordering across these.
	// (paneEnvUpdateMu and claudeEnvUpdateMu also have nested ordering with
	// tmux.CommandRouter locks — see nested lock ordering above.)
	//   windowMu, startupWarnMu, ctxMu,
	//   paneEnvUpdateMu, claudeEnvUpdateMu,
	//   snapshot.Service (internal locks: see snapshot.Service doc),
	//   scheduler.Service.mu (internal), scheduler.Service.templateMu (internal)
	//   orchestrator.Service.mu (internal)
	//   tmux.SessionManager.mu, tmux.CommandRouter.mu
	paneEnvUpdateMu         sync.Mutex
	paneEnvAppliedVersion   uint64
	claudeEnvUpdateMu       sync.Mutex
	claudeEnvAppliedVersion uint64
	workspace               string
	// launchDir is the working directory captured at startup. Read-only after
	// startup() returns; safe to access without mutex from any goroutine.
	launchDir          string
	startupWarnMu      sync.Mutex
	configLoadWarnings []string
	// Session lifecycle management (create, rename, kill, active session tracking).
	// Thread-safety is managed internally by the Service. No App-level mutex is needed.
	// Initialized in NewApp().
	sessionService *session.Service

	// Backend services.
	sessions   *tmux.SessionManager
	router     *tmux.CommandRouter
	pipeServer *ipc.PipeServer
	hotkeys    *hotkeys.Manager
	paneStates *panestate.Manager

	// MCP process management.
	// Independent locks: mcp.Registry.mu and mcp.Manager.mu are independent of
	// each other and of all other App-level locks.
	// mcpRegistry is retained for startup diagnostics and future config reloads.
	mcpRegistry      *mcp.Registry
	mcpManager       *mcp.Manager
	mcpBridgeCommand string

	// Window visibility state.
	windowMu       sync.Mutex
	windowVisible  bool
	windowToggling atomic.Bool // CAS guard to prevent concurrent toggleQuakeWindow
	shuttingDown   atomic.Bool // set true at the start of shutdown(); checked by worker recovery loops

	// wsHub provides a WebSocket binary stream for high-throughput pane data.
	// Set once during startup (single-goroutine); nil if WebSocket server fails to start.
	// Read by snapshotService flush callback (concurrent) and GetWebSocketURL (Wails-bound).
	// Safe without mutex: written once before any reader goroutine starts, never reassigned.
	wsHub *wsserver.Hub

	// Snapshot pipeline: pane output buffering, debounced snapshot emission,
	// delta computation, and metrics. Thread-safety is managed internally by
	// the Service. No App-level mutex is needed. Initialized in NewApp().
	snapshotService *snapshot.Service

	// Session log state (captures Warn/Error level records).
	// Thread-safety is managed internally by the Service. No App-level mutex is needed.
	// Initialized in NewApp(); ensureSessionLogService() provides a fallback for tests.
	sessionLogService     *sessionlog.Service
	sessionLogServiceOnce sync.Once

	// Input history state and behavior are encapsulated in internal/inputhistory.
	// Thread-safety is managed internally by the Service. No App-level mutex is needed.
	// Initialized in NewApp(); ensureInputHistoryService() provides a fallback for tests.
	inputHistoryService     *inputhistory.Service
	inputHistoryServiceOnce sync.Once

	// Pane scheduler state (multiple concurrent schedulers).
	// Thread-safety is managed internally by the Service. No App-level mutex is needed.
	// Initialized in NewApp().
	schedulerService *scheduler.Service

	// Task scheduler manager (per-session sequential task queue with completion detection).
	// Thread-safety is managed internally by the ServiceManager. No App-level mutex is needed.
	// Initialized in NewApp().
	taskSchedulerManager *taskscheduler.ServiceManager

	// Orchestrator team CRUD and launch operations.
	// Thread-safety is managed internally by the Service. No App-level mutex is needed.
	// Initialized in NewApp().
	orchestratorService *orchestrator.Service

	// Developer panel file browsing and git operations.
	// Stateless service; no mutex needed. Initialized in NewApp().
	devpanelService *devpanel.Service

	// Worktree lifecycle management (create, cleanup, status, commit/push).
	// Stateless service; no mutex needed. Initialized in NewApp().
	worktreeService *worktree.Service

	// MCP API operations (list, toggle, detail, stdio resolution).
	// Stateless service; no mutex needed. Initialized in NewApp().
	mcpAPIService *mcpapi.Service

	// sendKeys holds injectable functions for send-keys operations.
	// Initialized with defaultSendKeysIO() in NewApp().
	sendKeys sendKeysIO

	// openExplorerFn launches the file explorer for a given path.
	// Replaced in tests to avoid launching explorer.exe.
	openExplorerFn func(string) error

	// Background worker cancellation/waits.
	idleCancel context.CancelFunc
	bgWG       sync.WaitGroup
	setupWG    sync.WaitGroup
}

// NewApp creates the app service.
// All dependency wiring is delegated to buildXxxServiceDeps functions in app_wiring.go.
func NewApp() *App {
	app := &App{
		hotkeys:        hotkeys.NewManager(),
		paneStates:     panestate.NewManager(512 * 1024),
		configState:    config.NewStateService(),
		sendKeys:       defaultSendKeysIO(),
		openExplorerFn: openExplorer,
	}

	emitter := newAppRuntimeEventEmitterAdapter(app)
	isShuttingDown := func() bool { return app.shuttingDown.Load() }

	app.sessionLogService = sessionlog.NewService(emitter, isShuttingDown)
	app.inputHistoryService = inputhistory.NewService(emitter, isShuttingDown)
	app.sessionService = session.NewService(buildSessionServiceDeps(app))
	app.orchestratorService = orchestrator.NewService(buildOrchestratorServiceDeps(app))
	app.devpanelService = devpanel.NewService(buildDevPanelServiceDeps(app))
	app.worktreeService = worktree.NewService(buildWorktreeServiceDeps(app))
	app.mcpAPIService = mcpapi.NewService(buildMCPAPIServiceDeps(app))
	app.snapshotService = snapshot.NewService(buildSnapshotServiceDeps(app))
	app.schedulerService = scheduler.NewService(buildSchedulerServiceDeps(app))
	app.taskSchedulerManager = taskscheduler.NewServiceManager(buildTaskSchedulerDepsFactory(app))
	return app
}

// GetWebSocketURL returns the WebSocket endpoint URL for the frontend pane
// data stream. The frontend calls this on mount to establish a binary channel
// that bypasses Wails IPC overhead for high-frequency terminal output.
// Returns empty string if the WebSocket server is not available.
func (a *App) GetWebSocketURL() string {
	if a.wsHub == nil {
		slog.Debug("[WS] wsHub is nil, WebSocket URL unavailable")
		return ""
	}
	return a.wsHub.URL()
}
