package config

import (
	"sync"
	"sync/atomic"
	"time"
)

// UpdatedEvent is produced by StateService when a config save succeeds.
// The event carries the normalized config snapshot and a monotonic version
// that lets consumers detect and discard out-of-order deliveries.
type UpdatedEvent struct {
	Config             Config `json:"config"`
	Version            uint64 `json:"version"`
	UpdatedAtUnixMilli int64  `json:"updated_at_unix_milli"`
}

// StateService manages in-memory config state with thread-safe access,
// serialized persistence, and monotonic event versioning.
//
// Thread-safety:
//   - mu (RWMutex) protects the cfg field.
//   - saveMu (Mutex) serializes save operations.
//   - Lock ordering (outer -> inner): saveMu -> mu.
//   - eventVersion (atomic.Uint64) is independently safe.
//
// configPath is write-once during Initialize; safe to read without mutex
// after initialization completes.
type StateService struct {
	mu           sync.RWMutex
	saveMu       sync.Mutex
	eventVersion atomic.Uint64
	cfg          Config
	configPath   string
}

// NewStateService creates an uninitialized config state service.
// Call Initialize with a config path and initial config before using Save
// or Update.
func NewStateService() *StateService {
	return &StateService{}
}

// Initialize sets the config path and initial snapshot.
// Must be called exactly once during startup before any Save operations.
// Not safe for concurrent use — call from the startup sequence only.
// Panics if called more than once (programming error).
func (s *StateService) Initialize(configPath string, initial Config) {
	if s.configPath != "" {
		panic("config.StateService.Initialize called more than once")
	}
	s.configPath = configPath
	s.setSnapshotNoClone(Clone(initial))
}

// Snapshot returns a deep-copied config protected by RWMutex.
// All read access to the current config should go through this method.
func (s *StateService) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Clone(s.cfg)
}

// unsafeSnapshot returns the current config without cloning.
//
// WARNING: Reference-type fields (maps/slices/pointers) are shared with
// internal state. Callers MUST NOT modify the returned value or its nested
// fields. Use only for short-lived read-only access on the current goroutine.
// Callers that retain values or pass config data to long-lived goroutines
// must use Snapshot instead.
func (s *StateService) unsafeSnapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// SetSnapshot stores a deep-copied config protected by Mutex.
// Exported for test setup. Production code should use Save or Update.
func (s *StateService) SetSnapshot(cfg Config) {
	s.mu.Lock()
	s.cfg = Clone(cfg)
	s.mu.Unlock()
}

// setSnapshotNoClone stores cfg directly without cloning.
// REQUIRES: caller guarantees cfg is not shared with any other goroutine.
func (s *StateService) setSnapshotNoClone(cfg Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

// Save validates and persists cfg to disk, then updates the in-memory
// snapshot and bumps the monotonic event version.
//
// On success the returned UpdatedEvent carries a config that is safe for
// the caller to forward to event consumers without additional copying.
//
// On failure the in-memory snapshot and event version are unchanged.
func (s *StateService) Save(cfg Config) (UpdatedEvent, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return s.saveLocked(cfg)
}

// Update performs a read-modify-write cycle under saveMu.
// fn receives a snapshot of the current config and may mutate it freely.
// The modified config is then saved and an UpdatedEvent is returned.
func (s *StateService) Update(fn func(*Config)) (UpdatedEvent, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	current := s.Snapshot()
	fn(&current)
	return s.saveLocked(current)
}

// saveLocked persists cfg and updates the in-memory snapshot.
// REQUIRES: s.saveMu must be held by the caller.
func (s *StateService) saveLocked(cfg Config) (UpdatedEvent, error) {
	normalized, err := Save(s.configPath, cfg)
	if err != nil {
		return UpdatedEvent{}, err
	}
	// Clone once: internal snapshot gets the clone, event payload gets the
	// original normalized value. This is safe because Save() returns a fresh
	// value not shared with any other goroutine.
	s.setSnapshotNoClone(Clone(normalized))
	version := s.eventVersion.Add(1)

	return UpdatedEvent{
		Config:             normalized,
		Version:            version,
		UpdatedAtUnixMilli: time.Now().UnixMilli(),
	}, nil
}

// ConfigPath returns the current config file path.
// Returns empty string before Initialize is called.
func (s *StateService) ConfigPath() string {
	return s.configPath
}

// EventVersion returns the current monotonic event version.
func (s *StateService) EventVersion() uint64 {
	return s.eventVersion.Load()
}

// SetEventVersion sets the event version counter.
// Intended for test setup to control version numbering.
func (s *StateService) SetEventVersion(v uint64) {
	s.eventVersion.Store(v)
}
