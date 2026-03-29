package taskscheduler

import "sync"

// DepsFactory creates a Deps for a given session name.
type DepsFactory func(sessionName string) Deps

// ServiceManager manages per-session Service instances.
//
// Thread-safety is managed internally via mu. No external locking is required.
type ServiceManager struct {
	mu       sync.Mutex
	services map[string]*Service
	factory  DepsFactory
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
func (m *ServiceManager) Remove(sessionName string) {
	m.mu.Lock()
	svc, ok := m.services[sessionName]
	if ok {
		delete(m.services, sessionName)
	}
	m.mu.Unlock()

	if ok {
		svc.StopAll()
	}
}
