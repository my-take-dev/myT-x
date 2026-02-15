package panestate

import (
	"log/slog"
	"strings"
	"sync"
)

const (
	defaultCols = 120
	defaultRows = 40
)

// paneState holds per-pane terminal emulation state.
// Lock ordering: always acquire Manager.mu before paneState.mu.
type paneState struct {
	mu       sync.Mutex
	terminal *terminalState
	replay   replayRing
	cols     int
	rows     int
	dirty    bool
}

type replayRing struct {
	data []byte
	head int
	size int
}

func newReplayRing(capacity int) replayRing {
	if capacity <= 0 {
		capacity = 1
	}
	return replayRing{
		data: make([]byte, capacity),
	}
}

func (r *replayRing) write(chunk []byte) {
	if len(chunk) == 0 || len(r.data) == 0 {
		return
	}

	if len(chunk) >= len(r.data) {
		copy(r.data, chunk[len(chunk)-len(r.data):])
		r.head = 0
		r.size = len(r.data)
		return
	}

	n := copy(r.data[r.head:], chunk)
	if n < len(chunk) {
		copy(r.data, chunk[n:])
		r.head = len(chunk) - n
	} else {
		r.head = (r.head + n) % len(r.data)
	}

	r.size += len(chunk)
	if r.size > len(r.data) {
		r.size = len(r.data)
	}
}

func (r *replayRing) snapshot() []byte {
	if r.size == 0 {
		return nil
	}

	out := make([]byte, r.size)
	if r.size < len(r.data) {
		copy(out, r.data[:r.size])
		return out
	}

	n := copy(out, r.data[r.head:])
	copy(out[n:], r.data[:r.head])
	return out
}

// snapshotInto copies ring data into buf, growing it if needed.
// Returns the filled slice (may be a new allocation if buf capacity is insufficient).
func (r *replayRing) snapshotInto(buf []byte) []byte {
	if r.size == 0 {
		return buf[:0]
	}

	if cap(buf) < r.size {
		buf = make([]byte, r.size)
	} else {
		buf = buf[:r.size]
	}

	if r.size < len(r.data) {
		copy(buf, r.data[:r.size])
		return buf
	}

	n := copy(buf, r.data[r.head:])
	copy(buf[n:], r.data[:r.head])
	return buf
}

// Manager stores per-pane terminal state for lightweight resume.
// Lock ordering: Manager.mu (coarse) → paneState.mu (fine). Never reverse.
// RLock is used for read-only map lookups (Snapshot, Feed fast path for existing panes).
// Lock is used for map mutations (Feed slow path for new panes, RemovePane, RetainPanes).
type Manager struct {
	mu             sync.RWMutex
	maxReplayBytes int
	states         map[string]*paneState
	activePanes    map[string]struct{}
	replayPool     sync.Pool // reusable buffers for replay snapshot
}

// NewManager creates a new pane state manager.
func NewManager(maxReplayBytes int) *Manager {
	if maxReplayBytes <= 0 {
		maxReplayBytes = 512 * 1024
	}
	cap := maxReplayBytes
	return &Manager{
		maxReplayBytes: maxReplayBytes,
		states:         map[string]*paneState{},
		activePanes:    map[string]struct{}{},
		replayPool: sync.Pool{
			New: func() any {
				buf := make([]byte, 0, cap)
				return &buf
			},
		},
	}
}

// EnsurePane creates pane state if missing and applies size.
func (m *Manager) EnsurePane(paneID string, cols int, rows int) {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		slog.Warn("[DEBUG-PANESTATE] EnsurePane called with empty paneID, ignoring")
		return
	}
	cols, rows = sanitizeSize(cols, rows)

	m.mu.Lock()
	state := m.states[paneID]
	if state == nil {
		state = &paneState{
			terminal: newTerminalState(cols, rows),
			replay:   newReplayRing(m.maxReplayBytes),
			cols:     cols,
			rows:     rows,
		}
		m.states[paneID] = state
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// Per-pane update under pane lock (no manager lock held).
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.cols != cols || state.rows != rows {
		state.cols = cols
		state.rows = rows
		if !state.dirty {
			state.terminal.Resize(cols, rows)
		}
	}
}

// ResizePane resizes terminal state for a pane.
func (m *Manager) ResizePane(paneID string, cols int, rows int) {
	m.EnsurePane(paneID, cols, rows)
}

// Feed applies terminal output chunk to pane state.
func (m *Manager) Feed(paneID string, chunk []byte) {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" || len(chunk) == 0 {
		if paneID == "" && len(chunk) > 0 {
			slog.Warn("[DEBUG-PANESTATE] Feed called with empty paneID, ignoring")
		}
		return
	}

	// Phase 1: map lookup — read lock first (fast path for existing panes).
	var state *paneState
	var active bool
	m.mu.RLock()
	state = m.states[paneID]
	if state != nil {
		_, active = m.activePanes[paneID]
		m.mu.RUnlock()
	} else {
		m.mu.RUnlock()
		// Slow path: acquire write lock and double-check.
		m.mu.Lock()
		state = m.states[paneID]
		if state == nil {
			slog.Warn("[DEBUG-PANESTATE] Feed auto-creating pane with default size",
				"paneID", paneID, "cols", defaultCols, "rows", defaultRows)
			state = &paneState{
				terminal: newTerminalState(defaultCols, defaultRows),
				replay:   newReplayRing(m.maxReplayBytes),
				cols:     defaultCols,
				rows:     defaultRows,
			}
			m.states[paneID] = state
		}
		_, active = m.activePanes[paneID]
		m.mu.Unlock()
	}

	// Phase 2: per-pane work under pane lock (no manager lock held).
	state.mu.Lock()
	defer state.mu.Unlock()

	state.replay.write(chunk)
	if !active {
		state.dirty = true
		return
	}

	if state.dirty {
		m.rebuildTerminal(state)
		state.dirty = false
		return
	}
	if _, err := state.terminal.Write(chunk); err != nil {
		slog.Debug("[DEBUG-PANESTATE] failed to apply chunk to terminal state",
			"paneID", paneID,
			"chunkLen", len(chunk),
			"error", err,
		)
	}
}

// Snapshot returns a best-effort pane contents for restore.
func (m *Manager) Snapshot(paneID string) string {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return ""
	}

	// Phase 1: map lookup under read lock.
	m.mu.RLock()
	state := m.states[paneID]
	_, active := m.activePanes[paneID]
	m.mu.RUnlock()

	if state == nil {
		return ""
	}

	// Phase 2: per-pane read under pane lock.
	state.mu.Lock()
	defer state.mu.Unlock()

	if active {
		if state.dirty {
			m.rebuildTerminal(state)
			state.dirty = false
		}
		snapshot := strings.TrimRight(state.terminal.String(), "\n")
		if strings.TrimSpace(snapshot) != "" {
			return snapshot
		}
	}
	bp := m.replayPool.Get().(*[]byte)
	replay := state.replay.snapshotInto(*bp)
	result := string(replay)
	*bp = replay
	m.replayPool.Put(bp)
	return result
}

// RemovePane deletes state for one pane.
func (m *Manager) RemovePane(paneID string) {
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return
	}
	m.mu.Lock()
	delete(m.states, paneID)
	delete(m.activePanes, paneID)
	m.mu.Unlock()
}

// RetainPanes drops states not included in alive set.
func (m *Manager) RetainPanes(alive map[string]struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for paneID := range m.states {
		if _, ok := alive[paneID]; ok {
			continue
		}
		delete(m.states, paneID)
		delete(m.activePanes, paneID)
	}
}

// SetActivePanes updates active pane set and prepares emulation for active panes only.
func (m *Manager) SetActivePanes(active map[string]struct{}) {
	m.mu.Lock()

	next := make(map[string]struct{}, len(active))
	for paneID := range active {
		id := strings.TrimSpace(paneID)
		if id == "" {
			continue
		}
		next[id] = struct{}{}
	}
	m.activePanes = next

	// Collect ALL active pane states for dirty check outside manager lock.
	// We cannot check state.dirty here because it is protected by paneState.mu,
	// and reading it under Manager.mu alone would be a data race.
	var toRebuild []*paneState
	for paneID := range m.activePanes {
		state := m.states[paneID]
		if state != nil {
			toRebuild = append(toRebuild, state)
		}
	}
	m.mu.Unlock()

	// Rebuild under per-pane locks (no manager lock held).
	for _, state := range toRebuild {
		state.mu.Lock()
		if state.dirty {
			m.rebuildTerminal(state)
			state.dirty = false
		}
		state.mu.Unlock()
	}
}

// Reset clears all states.
func (m *Manager) Reset() {
	m.mu.Lock()
	m.states = map[string]*paneState{}
	m.activePanes = map[string]struct{}{}
	m.mu.Unlock()
}

func (m *Manager) rebuildTerminal(state *paneState) {
	cols, rows := sanitizeSize(state.cols, state.rows)
	state.terminal = newTerminalState(cols, rows)

	bp := m.replayPool.Get().(*[]byte)
	replay := state.replay.snapshotInto(*bp)
	if len(replay) > 0 {
		if _, err := state.terminal.Write(replay); err != nil {
			slog.Debug("[DEBUG-PANESTATE] failed to rebuild terminal from replay",
				"replayLen", len(replay),
				"error", err,
			)
		}
	}
	*bp = replay
	m.replayPool.Put(bp)
}
