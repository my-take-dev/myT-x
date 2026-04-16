package taskscheduler

import (
	"errors"
	"strings"
	"sync"
)

// DepsFactory creates a Deps for a given session name.
type DepsFactory func(sessionName string) Deps

// ServiceManager manages per-session Service instances.
//
// Thread-safety is managed internally via mu. No external locking is required.
type ServiceManager struct {
	mu       sync.Mutex
	services map[string]*Service
	factory  DepsFactory
	rename   func(*Service, string) error
}

// NewServiceManager creates a ServiceManager with the given factory.
// Panics if factory is nil.
func NewServiceManager(factory DepsFactory) *ServiceManager {
	if factory == nil {
		panic("taskscheduler.NewServiceManager: factory must be non-nil")
	}
	return &ServiceManager{
		services: make(map[string]*Service),
		factory:  factory,
		rename:   func(svc *Service, sessionName string) error { return svc.RenameSession(sessionName) },
	}
}

// GetOrCreate returns the Service for the given session, creating it if needed.
func (m *ServiceManager) GetOrCreate(sessionName string) *Service {
	m.mu.Lock()
	defer m.mu.Unlock()

	if svc, ok := m.services[sessionName]; ok {
		return svc
	}
	deps := m.factory(sessionName)
	svc := NewService(deps)
	m.services[sessionName] = svc
	return svc
}

// GetStatus returns the queue status for the given session.
// If no service exists, returns an idle status with empty items.
func (m *ServiceManager) GetStatus(sessionName string) QueueStatus {
	m.mu.Lock()
	svc, ok := m.services[sessionName]
	m.mu.Unlock()

	if !ok {
		return QueueStatus{
			Items:        []QueueItem{},
			RunStatus:    QueueIdle,
			CurrentIndex: -1,
			SessionName:  sessionName,
			GenerationID: "",
		}
	}
	return svc.GetStatus()
}

// StopAll stops all managed services. Used during application shutdown.
func (m *ServiceManager) StopAll() {
	m.mu.Lock()
	snapshot := make([]*Service, 0, len(m.services))
	for _, svc := range m.services {
		snapshot = append(snapshot, svc)
	}
	m.mu.Unlock()

	for _, svc := range snapshot {
		svc.StopAll()
	}
}

// Remove stops and removes the service for the given session.
// Retire is called inside the lock to prevent a concurrent GetOrCreate from
// recreating a service while the old one is still live.
func (m *ServiceManager) Remove(sessionName string) {
	m.mu.Lock()
	svc, ok := m.services[sessionName]
	if ok {
		svc.Retire()
		delete(m.services, sessionName)
	}
	m.mu.Unlock()

	if ok {
		svc.StopAll()
	}
}

// Rename rekeys a session service without discarding the existing queue state.
func (m *ServiceManager) Rename(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	m.mu.Lock()
	svc, ok := m.services[oldName]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	if _, exists := m.services[newName]; exists {
		m.mu.Unlock()
		return errors.New("task scheduler service already exists for renamed session")
	}
	if err := m.rename(svc, newName); err != nil {
		m.mu.Unlock()
		return err
	}
	delete(m.services, oldName)
	m.services[newName] = svc
	m.mu.Unlock()

	return nil
}
