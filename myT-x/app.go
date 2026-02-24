package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/hotkeys"
	"myT-x/internal/ipc"
	"myT-x/internal/panestate"
	"myT-x/internal/terminal"
	"myT-x/internal/tmux"
	"myT-x/internal/wsserver"
)

// App is the Wails-bound application service.
type App struct {
	// Runtime context lifecycle.
	ctx   context.Context
	ctxMu sync.RWMutex

	// Configuration state and startup warnings.
	// Lock ordering (outer -> inner):
	//   cfgSaveMu -> cfgMu
	//
	// Nested lock ordering (one-way only):
	//   paneEnvUpdateMu -> tmux.CommandRouter.paneEnvMu (via UpdatePaneEnv)
	//   claudeEnvUpdateMu -> tmux.CommandRouter.claudeEnvMu (via UpdateClaudeEnv)
	//
	// Lock ordering (outer -> inner):
	//   snapshotDeltaMu -> snapshotMu  (snapshotDelta acquires snapshotMu while holding snapshotDeltaMu)
	//
	// Independent locks: do not assume ordering across these.
	//   windowMu, outputMu, snapshotRequestMu, snapshotMetricsMu,
	//   startupWarnMu, activeSessMu, ctxMu, sessionLogMu,
	//   inputHistoryMu, inputLineBufMu
	//   tmux.SessionManager.mu, tmux.CommandRouter.mu
	//
	// Keep cfgSaveMu/cfgMu isolated from the independent lock set above.
	cfgMu                   sync.RWMutex
	cfgSaveMu               sync.Mutex
	configEventVersion      atomic.Uint64
	paneEnvUpdateMu         sync.Mutex
	paneEnvAppliedVersion   uint64
	claudeEnvUpdateMu       sync.Mutex
	claudeEnvAppliedVersion uint64
	cfg                     config.Config
	configPath              string
	workspace               string
	// launchDir is the working directory captured at startup. Read-only after
	// startup() returns; safe to access without mutex from any goroutine.
	launchDir          string
	startupWarnMu      sync.Mutex
	configLoadWarnings []string
	activeSessMu       sync.RWMutex
	activeSess         string

	// Backend services.
	sessions   *tmux.SessionManager
	router     *tmux.CommandRouter
	pipeServer *ipc.PipeServer
	hotkeys    *hotkeys.Manager
	paneStates *panestate.Manager

	// Window visibility state.
	windowMu       sync.Mutex
	windowVisible  bool
	windowToggling atomic.Bool // CAS guard to prevent concurrent toggleQuakeWindow
	shuttingDown   atomic.Bool // set true at the start of shutdown(); checked by worker recovery loops

	// Output buffering and snapshot state.
	outputMu      sync.Mutex
	outputFlusher *terminal.OutputFlushManager
	paneFeedCh    chan paneFeedItem
	paneFeedStop  context.CancelFunc

	// wsHub provides a WebSocket binary stream for high-throughput pane data.
	// Set once during startup (single-goroutine); nil if WebSocket server fails to start.
	// Read by ensureOutputFlusher flush callback (concurrent) and GetWebSocketURL (Wails-bound).
	// Safe without mutex: written once before any reader goroutine starts, never reassigned.
	wsHub *wsserver.Hub

	snapshotMu           sync.Mutex
	snapshotDeltaMu      sync.Mutex
	snapshotCache        map[string]tmux.SessionSnapshot
	snapshotPrimed       bool
	snapshotLastTopology uint64

	snapshotRequestMu         sync.Mutex
	snapshotRequestTimer      *time.Timer
	snapshotRequestGeneration uint64
	snapshotRequestDispatched uint64
	snapshotMetricsMu         sync.Mutex
	snapshotStats             snapshotMetrics

	// Session log state (captures Warn/Error level records).
	// Protected by sessionLogMu (RWMutex: write-lock for append/close, read-lock for get).
	//
	// sessionLogLastEmit: time of last app:session-log-updated emission; throttles
	//   high-frequency ping events to prevent Wails IPC saturation.
	// sessionLogSeq: monotonically increasing counter for stable frontend deduplication.
	sessionLogMu       sync.RWMutex
	sessionLogFile     *os.File
	sessionLogPath     string
	sessionLogEntries  sessionLogRingBuffer
	sessionLogLastEmit time.Time
	sessionLogSeq      uint64

	// Input history state (captures terminal input from SendInput/SendSyncInput).
	// Protected by inputHistoryMu (RWMutex: write-lock for append/close, read-lock for get).
	inputHistoryMu       sync.RWMutex
	inputHistoryFile     *os.File
	inputHistoryPath     string
	inputHistoryEntries  inputHistoryRingBuffer
	inputHistoryLastEmit time.Time
	inputHistorySeq      uint64

	// Input line buffering state (separate lock from inputHistoryMu to avoid
	// holding the history lock during timer management).
	// Lock ordering: inputLineBufMu is NEVER held while acquiring inputHistoryMu.
	inputLineBufMu   sync.Mutex
	inputLineBuffers map[string]*inputLineBuffer

	// Background worker cancellation/waits.
	idleCancel context.CancelFunc
	bgWG       sync.WaitGroup
	setupWG    sync.WaitGroup
}

type paneFeedItem struct {
	paneID  string
	chunk   []byte
	poolPtr *[]byte // original pool pointer for zero-alloc return
}

// NewApp creates the app service.
func NewApp() *App {
	return &App{
		// paneFeedCh buffer: 4096 items provides ~4 seconds of headroom at
		// 10 panes x 100 chunks/sec. When full, enqueuePaneStateFeed falls
		// back to direct Feed() call (see app_pane_feed.go).
		paneFeedCh:          make(chan paneFeedItem, 4096),
		snapshotCache:       map[string]tmux.SessionSnapshot{},
		hotkeys:             hotkeys.NewManager(),
		paneStates:          panestate.NewManager(512 * 1024),
		sessionLogEntries:   newSessionLogRingBuffer(sessionLogMaxEntries),
		inputHistoryEntries: newInputHistoryRingBuffer(inputHistoryMaxEntries),
		inputLineBuffers:    map[string]*inputLineBuffer{},
	}
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
