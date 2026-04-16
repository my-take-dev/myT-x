package mcp

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"sync"

	"myT-x/internal/singletaskrunner"
)

// ManagerConfig holds configuration for creating a Manager.
type ManagerConfig struct {
	Registry *Registry
	EmitFn   func(string, any)
	// ResolveWorkDir returns the working directory for the given session.
	// Used by session-scoped runtimes that need a filesystem root.
	ResolveWorkDir func(sessionName string) (string, error)
	NewPipeServer  func(MCPPipeConfig) managedPipeServer
	// SingleTaskRunnerManager starts and stops session-scoped single-task-runner runtimes.
	SingleTaskRunnerManager *singletaskrunner.ServiceManager
}

// Manager manages MCP instances across all sessions.
// Each session has independent MCP state.
//
// Lock ordering (outer -> inner):
//
//	Manager.mu -> instance.mu
//
// Registry.mu is independent from Manager.mu/instance.mu and is never held
// together with these locks in this package.
// emitMu is independent from Manager.mu/instance.mu and must never be
// acquired while any of those locks are held.
//
// INVARIANT: emitFn must be called OUTSIDE any lock to prevent deadlocks
// with the Wails runtime event system.
type Manager struct {
	mu       sync.RWMutex
	registry *Registry
	// sessions maps session name -> (mcp_id -> *instance).
	sessions map[string]map[string]*instance
	emitFn   func(name string, payload any)
	emitMu   sync.Mutex
	closed   bool
	// renaming tracks sessions currently being renamed.  Mutation methods
	// (SetEnabled, CleanupSession) wait for the rename to finish before
	// proceeding so they don't race with the pipe-restart / rollback window.
	renaming                map[string]chan struct{}
	resolveWorkDir          func(string) (string, error)
	newPipeServer           func(MCPPipeConfig) managedPipeServer
	singleTaskRunnerManager *singletaskrunner.ServiceManager
}

type managedPipeServer interface {
	Start() error
	Stop() error
	PipeName() string
}

type sessionRestartPlan struct {
	sessionName string
	mcpID       string
	inst        *instance
	def         Definition
	gen         uint64
	cancelFn    func() error
}

// instance holds the per-session, per-MCP runtime state.
type instance struct {
	mu     sync.RWMutex
	state  InstanceState
	cancel func() error // nil when stopped; calling it stops the running pipe server.
	pipe   managedPipeServer
	// generation is incremented each time SetEnabled changes state. It prevents
	// stale startInstance goroutines from clobbering a newer enable/disable cycle.
	generation uint64
}

// NewManager creates a Manager with the given config.
// The EmitFn is called outside of locks to emit state-change events to the frontend.
func NewManager(cfg ManagerConfig) *Manager {
	registry := cfg.Registry
	if registry == nil {
		slog.Warn("[WARN-MCP] NewManager called with nil registry, using empty registry")
		registry = NewRegistry()
	}
	emitFn := cfg.EmitFn
	if emitFn == nil {
		slog.Warn("[WARN-MCP] NewManager called with nil emitFn, events will be dropped")
		emitFn = func(string, any) {}
	}
	resolveWorkDir := cfg.ResolveWorkDir
	if resolveWorkDir == nil {
		resolveWorkDir = func(string) (string, error) {
			return ".", nil
		}
	}
	newPipeServer := cfg.NewPipeServer
	if newPipeServer == nil {
		newPipeServer = func(pipeCfg MCPPipeConfig) managedPipeServer {
			return NewMCPPipeServer(pipeCfg)
		}
	}
	return &Manager{
		registry:                registry,
		sessions:                make(map[string]map[string]*instance),
		emitFn:                  emitFn,
		renaming:                make(map[string]chan struct{}),
		resolveWorkDir:          resolveWorkDir,
		newPipeServer:           newPipeServer,
		singleTaskRunnerManager: cfg.SingleTaskRunnerManager,
	}
}

// waitForRename blocks until any in-progress rename touching sessionName
// completes.  It must be called WITHOUT holding m.mu.
func (m *Manager) waitForRename(sessionName string) {
	for {
		m.mu.RLock()
		ch, ok := m.renaming[sessionName]
		m.mu.RUnlock()
		if !ok {
			return
		}
		// Re-check the map after the wait in case a new rename started
		// immediately after the previous one completed.
		<-ch
	}
}

// SnapshotForSession returns all MCPSnapshots for a given session.
// If the session has no MCP state yet, snapshots are built from registry
// definitions with default state (disabled, stopped).
func (m *Manager) SnapshotForSession(sessionName string) ([]MCPSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}

	defs := m.registry.All()
	if len(defs) == 0 {
		return []MCPSnapshot{}, nil
	}

	instanceByID, err := func() (map[string]*instance, error) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.closed {
			return nil, fmt.Errorf("manager is closed")
		}
		sessionInstances := m.sessions[sessionName]
		if len(sessionInstances) == 0 {
			return nil, nil
		}
		copied := make(map[string]*instance, len(sessionInstances))
		maps.Copy(copied, sessionInstances)
		return copied, nil
	}()
	if err != nil {
		return nil, err
	}

	snapshots := make([]MCPSnapshot, 0, len(defs))
	for _, def := range defs {
		snap := defaultSnapshot(def)
		if inst := instanceByID[def.ID]; inst != nil {
			inst.mu.RLock()
			snap.Enabled = inst.state.Enabled
			snap.Status = inst.state.Status
			snap.Error = inst.state.Error
			if inst.pipe != nil {
				snap.PipePath = inst.pipe.PipeName()
			}
			inst.mu.RUnlock()
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots, nil
}

// SetEnabled toggles an MCP on/off for a session.
// Returns an error if the MCP ID is not found in the registry.
func (m *Manager) SetEnabled(sessionName, mcpID string, enabled bool) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return fmt.Errorf("session name is required")
	}
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		return fmt.Errorf("mcp ID is required")
	}

	// Wait for any in-progress rename to finish before mutating session state.
	m.waitForRename(sessionName)

	// Validate that the MCP ID exists in the registry.
	def, ok := m.registry.Get(mcpID)
	if !ok {
		return fmt.Errorf("unknown mcp ID %q", mcpID)
	}

	var (
		cancelFn     func() error
		stateChanged bool
		startNeeded  bool
		operationErr error
		inst         *instance
		startGen     uint64
	)
	func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed {
			operationErr = fmt.Errorf("manager is closed")
			return
		}
		sessionInstances := m.sessions[sessionName]
		if sessionInstances == nil {
			if enabled == def.DefaultEnabled {
				return
			}
			sessionInstances = make(map[string]*instance)
			m.sessions[sessionName] = sessionInstances
		}
		inst = sessionInstances[mcpID]
		if inst == nil {
			if enabled == def.DefaultEnabled {
				return
			}
			inst = &instance{
				state: InstanceState{
					MCPID:     mcpID,
					SessionID: sessionName,
					Enabled:   def.DefaultEnabled,
					Status:    StatusStopped,
				},
			}
			sessionInstances[mcpID] = inst
		}

		inst.mu.Lock()
		defer inst.mu.Unlock()
		if inst.state.Enabled == enabled {
			return
		}

		stateChanged = true
		inst.generation++
		inst.state.Enabled = enabled
		if enabled {
			inst.state.Status = StatusStarting
			inst.state.Error = ""
			startNeeded = true
			startGen = inst.generation
			return
		}

		// Stop the MCP pipe server if it's running.
		if inst.cancel != nil {
			cancelFn = inst.cancel
			inst.cancel = nil
		}
		inst.pipe = nil
		inst.state.Status = StatusStopped
		inst.state.Error = ""
	}()
	if operationErr != nil {
		return operationErr
	}

	if cancelFn != nil {
		if err := cancelFn(); err != nil {
			slog.Warn("[WARN-MCP] failed to stop MCP instance during disable",
				"session", sessionName,
				"mcp", mcpID,
				"error", err,
			)
		}
	}
	if !stateChanged {
		return nil
	}

	slog.Debug("[DEBUG-MCP] SetEnabled", "session", sessionName, "mcp", mcpID, "enabled", enabled)
	m.emitStateChanged(sessionName, mcpID)

	if startNeeded {
		go m.startInstance(sessionName, mcpID, inst, def, startGen)
	}

	return nil
}

// startInstance starts an MCPPipeServer for the given instance in the background.
// It resolves the session working directory, builds the pipe name, and transitions
// the instance status to Running or Error.
//
// The gen parameter is the generation counter at the time SetEnabled was called.
// If the instance's generation has changed by the time the pipe is ready, this
// goroutine is stale (a newer enable/disable cycle has taken over) and must
// stop the pipe and return without modifying the instance.
func (m *Manager) startInstance(sessionName, mcpID string, inst *instance, def Definition, gen uint64) {
	// Error is surfaced via StatusError state plus emitStateChanged inside
	// startInstanceSync, so the background launcher does not return it.
	_ = m.startInstanceSync(sessionName, mcpID, inst, def, gen, true)
}

func (m *Manager) startInstanceSync(sessionName, mcpID string, inst *instance, def Definition, gen uint64, emitState bool) error {
	rootDir, err := m.resolveWorkDir(sessionName)
	if err != nil {
		slog.Warn("[WARN-MCP] failed to resolve work dir for instance start",
			"session", sessionName, "mcp", mcpID, "error", err)
		inst.mu.Lock()
		if inst.generation != gen {
			inst.mu.Unlock()
			return nil
		}
		inst.state.Status = StatusError
		inst.state.Error = fmt.Sprintf("failed to resolve work dir: %v", err)
		inst.mu.Unlock()
		if emitState {
			m.emitStateChanged(sessionName, mcpID)
		}
		return fmt.Errorf("resolve work dir: %w", err)
	}

	if !instanceGenerationMatches(inst, gen) {
		return nil
	}

	pipeName := BuildMCPPipeName(sessionName, mcpID)
	pipeCfg, err := buildPipeConfig(pipeName, def, pipeConfigContext{
		rootDir:                 rootDir,
		sessionName:             sessionName,
		singleTaskRunnerManager: m.singleTaskRunnerManager,
	})
	if err != nil {
		slog.Warn("[WARN-MCP] failed to build pipe config",
			"session", sessionName, "mcp", mcpID, "pipe", pipeName, "error", err)
		inst.mu.Lock()
		if inst.generation != gen {
			inst.mu.Unlock()
			return nil
		}
		inst.state.Status = StatusError
		inst.state.Error = fmt.Sprintf("failed to build pipe config: %v", err)
		inst.mu.Unlock()
		if emitState {
			m.emitStateChanged(sessionName, mcpID)
		}
		return fmt.Errorf("build pipe config: %w", err)
	}
	pipe := m.newPipeServer(pipeCfg)

	if err := pipe.Start(); err != nil {
		if stopErr := pipe.Stop(); stopErr != nil {
			slog.Warn("[WARN-MCP] failed to stop pipe server after start error",
				"session", sessionName, "mcp", mcpID, "pipe", pipeName, "error", stopErr)
		}
		slog.Warn("[WARN-MCP] failed to start pipe server",
			"session", sessionName, "mcp", mcpID, "pipe", pipeName, "error", err)
		inst.mu.Lock()
		if inst.generation != gen {
			inst.mu.Unlock()
			return nil
		}
		inst.state.Status = StatusError
		inst.state.Error = fmt.Sprintf("failed to start pipe: %v", err)
		inst.mu.Unlock()
		if emitState {
			m.emitStateChanged(sessionName, mcpID)
		}
		return fmt.Errorf("start pipe: %w", err)
	}

	inst.mu.Lock()
	// Check that the instance generation hasn't changed (could have been
	// toggled off/on again while we were starting).
	if inst.generation != gen {
		inst.mu.Unlock()
		if stopErr := pipe.Stop(); stopErr != nil {
			slog.Warn("[WARN-MCP] failed to stop stale pipe server",
				"session", sessionName, "mcp", mcpID, "pipe", pipeName, "error", stopErr)
		}
		return nil
	}
	inst.state.Status = StatusRunning
	inst.state.Error = ""
	inst.pipe = pipe
	inst.cancel = func() error { return pipe.Stop() }
	inst.mu.Unlock()

	slog.Debug("[DEBUG-MCP] instance started",
		"session", sessionName, "mcp", mcpID, "pipe", pipeName)
	if emitState {
		m.emitStateChanged(sessionName, mcpID)
	}
	return nil
}

// GetDetail returns the full detail for one MCP in a session.
func (m *Manager) GetDetail(sessionName, mcpID string) (MCPSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return MCPSnapshot{}, fmt.Errorf("session name is required")
	}
	mcpID = strings.TrimSpace(mcpID)
	if mcpID == "" {
		return MCPSnapshot{}, fmt.Errorf("mcp ID is required")
	}

	def, ok := m.registry.Get(mcpID)
	if !ok {
		return MCPSnapshot{}, fmt.Errorf("unknown mcp ID %q", mcpID)
	}

	inst, err := func() (*instance, error) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.closed {
			return nil, fmt.Errorf("manager is closed")
		}
		sessionInstances := m.sessions[sessionName]
		if sessionInstances == nil {
			return nil, nil
		}
		return sessionInstances[mcpID], nil
	}()
	if err != nil {
		return MCPSnapshot{}, err
	}

	snap := defaultSnapshot(def)
	if inst != nil {
		inst.mu.RLock()
		snap.Enabled = inst.state.Enabled
		snap.Status = inst.state.Status
		snap.Error = inst.state.Error
		if inst.pipe != nil {
			snap.PipePath = inst.pipe.PipeName()
		}
		inst.mu.RUnlock()
	}

	return snap, nil
}

// CleanupSession removes all MCP state for a destroyed session.
// Any running processes are stopped before cleanup.
func (m *Manager) CleanupSession(sessionName string) error {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil
	}

	var (
		sessionInstances map[string]*instance
		ok               bool
	)
	for {
		// Wait for any in-progress rename to finish before cleaning up session state.
		m.waitForRename(sessionName)

		m.mu.Lock()
		if renameDone, renaming := m.renaming[sessionName]; renaming {
			m.mu.Unlock()
			<-renameDone
			continue
		}
		if m.closed {
			m.mu.Unlock()
			slog.Debug("[DEBUG-MCP] CleanupSession skipped because manager is closed", "session", sessionName)
			return nil
		}
		sessionInstances, ok = m.sessions[sessionName]
		if ok {
			delete(m.sessions, sessionName)
		}
		m.mu.Unlock()
		break
	}

	if !ok {
		return nil
	}

	// Stop all running instances for this session.
	var cleanupErrs []error
	for mcpID, inst := range sessionInstances {
		cancelFn := stopInstance(inst, true)
		if cancelFn != nil {
			if err := cancelFn(); err != nil {
				wrappedErr := fmt.Errorf("stop mcp %q: %w", mcpID, err)
				slog.Warn("[WARN-MCP] failed to stop MCP instance during cleanup",
					"session", sessionName,
					"mcp", mcpID,
					"error", err,
				)
				cleanupErrs = append(cleanupErrs, wrappedErr)
			}
		}
	}

	slog.Debug("[DEBUG-MCP] CleanupSession", "session", sessionName)
	return errors.Join(cleanupErrs...)
}

// RenameSession migrates MCP runtime state to a renamed session.
// Running or starting instances are restarted so their pipe names and session
// identifiers stay consistent with the new session name.
func (m *Manager) RenameSession(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	if oldName == "" {
		return fmt.Errorf("old session name is required")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("new session name is required")
	}
	if oldName == newName {
		return nil
	}

	var restartPlans []sessionRestartPlan
	var restartIDs []string

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return fmt.Errorf("manager is closed")
	}

	sessionInstances, ok := m.sessions[oldName]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	if _, exists := m.sessions[newName]; exists {
		m.mu.Unlock()
		return fmt.Errorf("session %q already has MCP state", newName)
	}

	defsByID := make(map[string]Definition)
	for mcpID, inst := range sessionInstances {
		restartNeeded := func() bool {
			inst.mu.Lock()
			defer inst.mu.Unlock()
			return inst.state.Enabled && (inst.cancel != nil || inst.pipe != nil || inst.state.Status == StatusStarting)
		}()
		if !restartNeeded {
			continue
		}

		def, exists := m.registry.Get(mcpID)
		if !exists {
			m.mu.Unlock()
			return fmt.Errorf("unknown mcp ID %q", mcpID)
		}
		defsByID[mcpID] = def
	}

	delete(m.sessions, oldName)
	m.sessions[newName] = sessionInstances

	// Mark both session names as renaming so concurrent SetEnabled /
	// CleanupSession calls block until the restart / rollback phase completes.
	renameDone := make(chan struct{})
	m.renaming[oldName] = renameDone
	m.renaming[newName] = renameDone

	for mcpID, inst := range sessionInstances {
		func() {
			inst.mu.Lock()
			defer inst.mu.Unlock()

			inst.state.SessionID = newName

			restartNeeded := inst.state.Enabled && (inst.cancel != nil || inst.pipe != nil || inst.state.Status == StatusStarting)
			if restartNeeded {
				inst.generation++
				gen := inst.generation
				cancelFn := inst.cancel
				inst.cancel = nil
				inst.pipe = nil
				inst.state.Status = StatusStarting
				inst.state.Error = ""
				restartPlans = append(restartPlans, sessionRestartPlan{
					sessionName: newName,
					mcpID:       mcpID,
					inst:        inst,
					def:         defsByID[mcpID],
					gen:         gen,
					cancelFn:    cancelFn,
				})
				restartIDs = append(restartIDs, mcpID)
			}
		}()
	}
	m.mu.Unlock()

	// Clear the renaming guard when we return, regardless of success or rollback.
	defer func() {
		m.mu.Lock()
		delete(m.renaming, oldName)
		delete(m.renaming, newName)
		m.mu.Unlock()
		close(renameDone)
	}()

	for _, plan := range restartPlans {
		if plan.cancelFn != nil {
			if err := plan.cancelFn(); err != nil {
				slog.Warn("[WARN-MCP] failed to stop renamed MCP before restart",
					"session", newName,
					"mcp", plan.mcpID,
					"error", err,
				)
			}
		}
	}
	for _, plan := range restartPlans {
		if err := m.startInstanceSync(plan.sessionName, plan.mcpID, plan.inst, plan.def, plan.gen, false); err != nil {
			rollbackErr := m.rollbackRenamedSession(oldName, newName, sessionInstances, restartPlans)
			if rollbackErr != nil {
				return fmt.Errorf("restart renamed MCP %q: %w (rollback failed: %v)", plan.mcpID, err, rollbackErr)
			}
			return fmt.Errorf("restart renamed MCP %q: %w", plan.mcpID, err)
		}
	}
	for _, mcpID := range restartIDs {
		m.emitStateChanged(newName, mcpID)
	}

	slog.Debug("[DEBUG-MCP] RenameSession",
		"oldSession", oldName,
		"newSession", newName,
		"restartCount", len(restartPlans),
	)
	return nil
}

func (m *Manager) rollbackRenamedSession(
	oldName, newName string,
	sessionInstances map[string]*instance,
	restartPlans []sessionRestartPlan,
) error {
	restartIDs := make([]string, 0, len(restartPlans))

	m.mu.Lock()
	if _, ok := m.sessions[newName]; ok {
		delete(m.sessions, newName)
	}
	if _, exists := m.sessions[oldName]; !exists {
		m.sessions[oldName] = sessionInstances
	}
	m.mu.Unlock()

	for _, inst := range sessionInstances {
		inst.mu.Lock()
		inst.state.SessionID = oldName
		inst.mu.Unlock()
	}

	var rollbackErrs []error
	for _, plan := range restartPlans {
		inst := plan.inst

		inst.mu.Lock()
		cancelFn := inst.cancel
		inst.cancel = nil
		inst.pipe = nil
		inst.generation++
		rollbackGen := inst.generation
		inst.state.SessionID = oldName
		inst.state.Status = StatusStarting
		inst.state.Error = ""
		inst.mu.Unlock()

		if cancelFn != nil {
			if err := cancelFn(); err != nil {
				slog.Warn("[WARN-MCP] failed to stop renamed MCP during rollback",
					"session", oldName,
					"mcp", plan.mcpID,
					"error", err,
				)
			}
		}
		restartIDs = append(restartIDs, plan.mcpID)

		if err := m.startInstanceSync(oldName, plan.mcpID, inst, plan.def, rollbackGen, false); err != nil {
			inst.mu.Lock()
			if inst.state.Status == StatusStarting {
				inst.state.Status = StatusError
				inst.state.Error = fmt.Sprintf("rollback restart failed: %v", err)
			}
			inst.mu.Unlock()
			slog.Error("[ERROR-MCP] rollback instance restart failed",
				"session", oldName,
				"mcp", plan.mcpID,
				"error", err,
			)
			rollbackErrs = append(rollbackErrs, fmt.Errorf("%s: %w", plan.mcpID, err))
		}
	}

	for _, mcpID := range restartIDs {
		m.emitStateChanged(oldName, mcpID)
	}
	if len(rollbackErrs) > 0 {
		return errors.Join(rollbackErrs...)
	}
	return nil
}

// Close stops all MCP instances across all sessions.
func (m *Manager) Close() {
	m.close(true)
}

// CloseWithoutEvent stops all MCP instances without emitting lifecycle events.
// This is used by App shutdown to avoid depending on frontend runtime state.
func (m *Manager) CloseWithoutEvent() {
	m.close(false)
}

func (m *Manager) close(emitLifecycleEvent bool) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	allSessions := m.sessions
	m.sessions = make(map[string]map[string]*instance)
	m.mu.Unlock()

	for sessionName, sessionInstances := range allSessions {
		for _, inst := range sessionInstances {
			cancelFn := stopInstance(inst, true)
			if cancelFn != nil {
				if err := cancelFn(); err != nil {
					slog.Warn("[WARN-MCP] failed to stop MCP instance during close",
						"session", sessionName,
						"error", err,
					)
				}
			}
		}
		slog.Debug("[DEBUG-MCP] Close: cleaned session", "session", sessionName)
	}
	if !emitLifecycleEvent {
		return
	}

	// Emit lifecycle event outside locks so the frontend can reconcile state.
	m.emitMu.Lock()
	m.emitFn("mcp:manager-closed", nil)
	m.emitMu.Unlock()
}

func (m *Manager) emitStateChanged(sessionName, mcpID string) {
	m.emitMu.Lock()
	defer m.emitMu.Unlock()

	m.mu.RLock()
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return
	}

	m.emitFn("mcp:state-changed", map[string]any{
		"session_name": sessionName,
		"mcp_id":       mcpID,
	})
}

func defaultSnapshot(def Definition) Snapshot {
	return Snapshot{
		ID:           def.ID,
		Name:         def.Name,
		Description:  def.Description,
		Enabled:      def.DefaultEnabled,
		Status:       StatusStopped,
		UsageSample:  def.UsageSample,
		ConfigParams: cloneConfigParams(def.ConfigParams),
		Kind:         def.Kind,
	}
}

func instanceGenerationMatches(inst *instance, gen uint64) bool {
	inst.mu.RLock()
	defer inst.mu.RUnlock()
	return inst.generation == gen
}

func stopInstance(inst *instance, invalidateGeneration bool) func() error {
	var cancelFn func() error
	inst.mu.Lock()
	if invalidateGeneration {
		inst.generation++
	}
	if inst.cancel != nil {
		cancelFn = inst.cancel
		inst.cancel = nil
	}
	inst.pipe = nil
	inst.state.Enabled = false
	inst.state.Status = StatusStopped
	inst.state.Error = ""
	inst.mu.Unlock()
	return cancelFn
}
