//go:build !windows

package hotkeys

import (
	"errors"
	"log/slog"
	"sync"
)

// Manager manages one global hotkey registration.
type Manager struct {
	mu     sync.Mutex
	active string
}

// NewManager creates a new hotkey manager.
func NewManager() *Manager {
	return &Manager{}
}

// Start validates the binding spec. On non-Windows targets, the binding is
// parsed but no OS-level hotkey is registered. The onTrigger callback will
// never fire; callers should check the platform if hotkey functionality is
// required.
func (m *Manager) Start(spec string, onTrigger func()) error {
	if onTrigger == nil {
		return errors.New("onTrigger callback is required")
	}
	binding, err := ParseBinding(spec)
	if err != nil {
		return err
	}

	slog.Warn("[hotkey] DEBUG global hotkeys are not supported on this platform; binding validated but will never fire",
		"binding", binding.Normalized())

	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = binding.Normalized()
	return nil
}

// Stop unregisters the active global hotkey.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = ""
	return nil
}

// ActiveBinding returns the normalized binding string for the active hotkey.
func (m *Manager) ActiveBinding() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}
