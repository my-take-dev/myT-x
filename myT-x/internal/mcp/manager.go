package mcp

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

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
}

// instance holds the per-session, per-MCP runtime state.
type instance struct {
	mu     sync.RWMutex
	state  InstanceState
	cancel func() // nil when stopped; set when process lifecycle is wired.
}

// NewManager creates a Manager with the given registry and event emission callback.
// The emitFn is called outside of locks to emit state-change events to the frontend.
func NewManager(registry *Registry, emitFn func(string, any)) *Manager {
	if registry == nil {
		slog.Warn("[WARN-MCP] NewManager called with nil registry, using empty registry")
		registry = NewRegistry()
	}
	if emitFn == nil {
		// NOTE: Defensive fallback — should not happen in production.
		// Using a no-op emitter prevents nil pointer panics.
		slog.Warn("[WARN-MCP] NewManager called with nil emitFn, events will be dropped")
		emitFn = func(string, any) {}
	}
	return &Manager{
		registry: registry,
		sessions: make(map[string]map[string]*instance),
		emitFn:   emitFn,
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
		for id, inst := range sessionInstances {
			copied[id] = inst
		}
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

	// Validate that the MCP ID exists in the registry.
	def, ok := m.registry.Get(mcpID)
	if !ok {
		return fmt.Errorf("unknown mcp ID %q", mcpID)
	}

	var (
		cancelFn     func()
		stateChanged bool
		operationErr error
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
		inst := sessionInstances[mcpID]
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
		inst.state.Enabled = enabled
		if enabled {
			// NOTE: Actual process start will be implemented in a future step.
			// For now, just mark the status as "stopped" (process not yet managed).
			inst.state.Status = StatusStopped
			inst.state.Error = ""
			return
		}

		// Stop the MCP process if it's running.
		if inst.cancel != nil {
			cancelFn = inst.cancel
			inst.cancel = nil
		}
		inst.state.Status = StatusStopped
		inst.state.Error = ""
	}()
	if operationErr != nil {
		return operationErr
	}

	if cancelFn != nil {
		// Fire-and-forget stop signal: caller does not wait for process teardown.
		cancelFn()
	}
	if !stateChanged {
		return nil
	}

	// Emit state change event OUTSIDE locks (#37, #41).
	slog.Debug("[DEBUG-MCP] SetEnabled", "session", sessionName, "mcp", mcpID, "enabled", enabled)
	m.emitStateChanged(sessionName, mcpID)

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
		inst.mu.RUnlock()
	}

	return snap, nil
}

// CleanupSession removes all MCP state for a destroyed session.
// Any running processes are stopped before cleanup.
func (m *Manager) CleanupSession(sessionName string) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		slog.Debug("[DEBUG-MCP] CleanupSession skipped because manager is closed", "session", sessionName)
		return
	}
	sessionInstances, ok := m.sessions[sessionName]
	if ok {
		delete(m.sessions, sessionName)
	}
	m.mu.Unlock()

	if !ok {
		return
	}

	// Stop all running instances for this session.
	for _, inst := range sessionInstances {
		var cancelFn func()
		inst.mu.Lock()
		if inst.cancel != nil {
			cancelFn = inst.cancel
			inst.cancel = nil
		}
		inst.state.Status = StatusStopped
		inst.state.Error = ""
		inst.mu.Unlock()
		if cancelFn != nil {
			// Fire-and-forget cleanup; we intentionally do not block for teardown.
			cancelFn()
		}
	}

	// No frontend event is emitted here: session removal itself is driven by tmux
	// lifecycle events, and MCP state for the removed session is no longer visible.
	slog.Debug("[DEBUG-MCP] CleanupSession", "session", sessionName)
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
			var cancelFn func()
			inst.mu.Lock()
			if inst.cancel != nil {
				cancelFn = inst.cancel
				inst.cancel = nil
			}
			inst.state.Status = StatusStopped
			inst.state.Error = ""
			inst.mu.Unlock()
			if cancelFn != nil {
				cancelFn()
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
	}
}
