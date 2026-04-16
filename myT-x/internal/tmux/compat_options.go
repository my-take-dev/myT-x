package tmux

import (
	"strings"
	"sync"
)

const compatOptionFocusEvents = "focus-events"

type compatOptionScopeKind string

const (
	compatOptionScopeGlobal  compatOptionScopeKind = "global"
	compatOptionScopeSession compatOptionScopeKind = "session"
	compatOptionScopeWindow  compatOptionScopeKind = "window"
	compatOptionScopePane    compatOptionScopeKind = "pane"
)

type compatOptionScope struct {
	kind      compatOptionScopeKind
	sessionID int
	windowID  int
	paneID    int
}

type compatOptionStore struct {
	mu       sync.RWMutex
	global   map[string]string
	sessions map[int]map[string]string
	windows  map[int]map[string]string
	panes    map[int]map[string]string
}

func newCompatOptionStore() *compatOptionStore {
	return &compatOptionStore{
		global:   make(map[string]string),
		sessions: make(map[int]map[string]string),
		windows:  make(map[int]map[string]string),
		panes:    make(map[int]map[string]string),
	}
}

func supportedCompatOptionNames() []string {
	return []string{compatOptionFocusEvents}
}

func compatOptionDefaultValue(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case compatOptionFocusEvents:
		return "off", true
	default:
		return "", false
	}
}

func normalizeCompatOptionValue(name string, value string) (string, bool) {
	switch strings.TrimSpace(name) {
	case compatOptionFocusEvents:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "on", "true":
			return "on", true
		case "0", "off", "false":
			return "off", true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func (s *compatOptionStore) getOption(scope compatOptionScope, name string) (string, bool) {
	defaultValue, supported := compatOptionDefaultValue(name)
	if !supported {
		return "", false
	}

	trimmedName := strings.TrimSpace(name)
	s.mu.RLock()
	value, ok := s.getOptionExactLocked(scope, trimmedName)
	if !ok {
		value, ok = s.getInheritedOptionLocked(scope, trimmedName)
	}
	s.mu.RUnlock()
	if ok {
		return value, true
	}
	return defaultValue, true
}

func (s *compatOptionStore) setOption(scope compatOptionScope, name string, value string, onlyIfUnset bool) bool {
	normalizedValue, ok := normalizeCompatOptionValue(name, value)
	if !ok {
		return false
	}

	trimmedName := strings.TrimSpace(name)
	s.mu.Lock()
	scopeMap := s.ensureScopeMapLocked(scope)
	if onlyIfUnset {
		if _, exists := scopeMap[trimmedName]; exists {
			s.mu.Unlock()
			return true
		}
	}
	scopeMap[trimmedName] = normalizedValue
	s.mu.Unlock()
	return true
}

func (s *compatOptionStore) unsetOption(scope compatOptionScope, name string) bool {
	trimmedName := strings.TrimSpace(name)
	if _, supported := compatOptionDefaultValue(trimmedName); !supported {
		return false
	}

	s.mu.Lock()
	scopeMap := s.scopeMapLocked(scope)
	if scopeMap != nil {
		delete(scopeMap, trimmedName)
	}
	s.mu.Unlock()
	return true
}

func (s *compatOptionStore) getOptionExactLocked(scope compatOptionScope, name string) (string, bool) {
	scopeMap := s.scopeMapLocked(scope)
	if scopeMap == nil {
		return "", false
	}
	value, ok := scopeMap[name]
	return value, ok
}

func (s *compatOptionStore) getInheritedOptionLocked(scope compatOptionScope, name string) (string, bool) {
	switch scope.kind {
	case compatOptionScopePane:
		if value, ok := s.lookupScopeValueLocked(compatOptionScope{kind: compatOptionScopeWindow, windowID: scope.windowID}, name); ok {
			return value, true
		}
		if value, ok := s.lookupScopeValueLocked(compatOptionScope{kind: compatOptionScopeSession, sessionID: scope.sessionID}, name); ok {
			return value, true
		}
	case compatOptionScopeWindow:
		if value, ok := s.lookupScopeValueLocked(compatOptionScope{kind: compatOptionScopeSession, sessionID: scope.sessionID}, name); ok {
			return value, true
		}
	case compatOptionScopeSession:
	}
	return s.lookupScopeValueLocked(compatOptionScope{kind: compatOptionScopeGlobal}, name)
}

func (s *compatOptionStore) lookupScopeValueLocked(scope compatOptionScope, name string) (string, bool) {
	scopeMap := s.scopeMapLocked(scope)
	if scopeMap == nil {
		return "", false
	}
	value, ok := scopeMap[name]
	return value, ok
}

func (s *compatOptionStore) ensureScopeMapLocked(scope compatOptionScope) map[string]string {
	switch scope.kind {
	case compatOptionScopeGlobal:
		return s.global
	case compatOptionScopeSession:
		if s.sessions[scope.sessionID] == nil {
			s.sessions[scope.sessionID] = make(map[string]string)
		}
		return s.sessions[scope.sessionID]
	case compatOptionScopeWindow:
		if s.windows[scope.windowID] == nil {
			s.windows[scope.windowID] = make(map[string]string)
		}
		return s.windows[scope.windowID]
	case compatOptionScopePane:
		if s.panes[scope.paneID] == nil {
			s.panes[scope.paneID] = make(map[string]string)
		}
		return s.panes[scope.paneID]
	default:
		return nil
	}
}

func (s *compatOptionStore) scopeMapLocked(scope compatOptionScope) map[string]string {
	switch scope.kind {
	case compatOptionScopeGlobal:
		return s.global
	case compatOptionScopeSession:
		return s.sessions[scope.sessionID]
	case compatOptionScopeWindow:
		return s.windows[scope.windowID]
	case compatOptionScopePane:
		return s.panes[scope.paneID]
	default:
		return nil
	}
}
