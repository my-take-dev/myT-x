package main

import (
	"context"
	"sync"
	"sync/atomic"

	"myT-x/internal/config"
	"myT-x/internal/hotkeys"
	"myT-x/internal/ipc"
	"myT-x/internal/panestate"
	"myT-x/internal/terminal"
	"myT-x/internal/tmux"
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
	//
	// Independent locks: do not assume ordering across these.
	//   windowMu, outputMu, snapshotMu, startupWarnMu, activeSessMu, ctxMu
	//   tmux.SessionManager.mu, tmux.CommandRouter.mu
	//
	// Keep cfgSaveMu/cfgMu isolated from the independent lock set above.
	cfgMu                 sync.RWMutex
	cfgSaveMu             sync.Mutex
	configEventVersion    atomic.Uint64
	paneEnvUpdateMu       sync.Mutex
	paneEnvAppliedVersion uint64
	cfg                   config.Config
	configPath            string
	workspace             string
	startupWarnMu         sync.Mutex
	configLoadWarnings    []string
	activeSessMu          sync.RWMutex
	activeSess            string

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

	// Output buffering and snapshot state.
	outputMu       sync.Mutex
	outputBuffers  map[string]*terminal.OutputBuffer
	paneFeedCh     chan paneFeedItem
	paneFeedStop   context.CancelFunc
	snapshotMu     sync.Mutex
	snapshotCache  map[string]tmux.SessionSnapshot
	snapshotPrimed bool
	snapshotStats  snapshotMetrics

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
		outputBuffers: map[string]*terminal.OutputBuffer{},
		paneFeedCh:    make(chan paneFeedItem, 4096),
		snapshotCache: map[string]tmux.SessionSnapshot{},
		hotkeys:       hotkeys.NewManager(),
		paneStates:    panestate.NewManager(512 * 1024),
	}
}
