package tmux

import (
	"sync"
	"time"
)

// SessionManager owns session/window/pane state.
type SessionManager struct {
	sessions map[string]*TmuxSession
	panes    map[int]*TmuxPane

	nextSessionID int
	nextPaneID    int
	nextWindowID  int
	now           func() time.Time
	idleThreshold time.Duration

	// generation increments on any state mutation.
	generation uint64
	// topologyGeneration increments when session/window/pane topology changes.
	topologyGeneration uint64
	sortedSessionNames []string
	sortedNamesDirty   bool
	// snapshotGeneration is the generation number of snapshotCache.
	snapshotGeneration uint64
	snapshotCache      []SessionSnapshot
	mu                 sync.RWMutex
}

// NewSessionManager creates a SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:         map[string]*TmuxSession{},
		panes:            map[int]*TmuxPane{},
		now:              time.Now,
		idleThreshold:    5 * time.Second,
		sortedNamesDirty: true,
	}
}

func (m *SessionManager) markStateMutationLocked() {
	m.generation++
}

func (m *SessionManager) markTopologyMutationLocked() {
	m.generation++
	m.topologyGeneration++
}

func (m *SessionManager) markSessionMapMutationLocked() {
	m.sortedNamesDirty = true
	m.generation++
	m.topologyGeneration++
}

// TopologyGeneration returns a monotonically increasing counter that changes
// when session/window/pane topology changes.
func (m *SessionManager) TopologyGeneration() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.topologyGeneration
}
