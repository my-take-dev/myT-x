package tmux

import (
	"fmt"
	"sync"
	"testing"
)

type captureEmitter struct {
	mu     sync.Mutex
	events []capturedEvent
}

type capturedEvent struct {
	name    string
	payload any
}

func (e *captureEmitter) Emit(name string, payload any) {
	e.mu.Lock()
	e.events = append(e.events, capturedEvent{name: name, payload: payload})
	e.mu.Unlock()
}

func (e *captureEmitter) Events() []capturedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]capturedEvent, len(e.events))
	copy(cp, e.events)
	return cp
}

func (e *captureEmitter) EventNames() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := make([]string, 0, len(e.events))
	for _, event := range e.events {
		names = append(names, event.name)
	}
	return names
}

// injectTestWindow creates and injects a second window with a single pane into
// the specified session. This bypasses the removed AddWindow API by directly
// manipulating SessionManager internals under lock.
//
// Returns the injected window and its sole pane for use in assertions.
// Callers must NOT hold manager.mu when calling this function.
func injectTestWindow(t *testing.T, manager *SessionManager, sessionName, windowName string) (*TmuxWindow, *TmuxPane) {
	t.Helper()

	manager.mu.Lock()
	defer manager.mu.Unlock()

	session, ok := manager.sessions[sessionName]
	if !ok {
		t.Fatalf("injectTestWindow: session %q not found", sessionName)
	}

	window := &TmuxWindow{
		ID:       manager.nextWindowID,
		Name:     windowName,
		ActivePN: 0,
		Session:  session,
	}
	manager.nextWindowID++

	pane := &TmuxPane{
		ID:       manager.nextPaneID,
		idString: fmt.Sprintf("%%%d", manager.nextPaneID),
		Index:    0,
		Active:   true,
		Width:    120,
		Height:   40,
		Env:      map[string]string{},
		Window:   window,
	}
	manager.nextPaneID++

	window.Panes = []*TmuxPane{pane}
	window.Layout = newLeafLayout(pane.ID)
	session.Windows = append(session.Windows, window)
	manager.panes[pane.ID] = pane

	return window, pane
}
