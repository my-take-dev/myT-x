package tmux

import "testing"

func seededPaneOutputHistory(data string) *PaneOutputHistory {
	history := NewPaneOutputHistory(64)
	history.Write([]byte(data))
	return history
}

func assertPaneHistoryReleased(t *testing.T, pane *TmuxPane, history *PaneOutputHistory) {
	t.Helper()
	if pane.OutputHistory != nil {
		t.Fatal("pane OutputHistory should be nil after cleanup")
	}
	if got := history.Capture(); got != nil {
		t.Fatalf("released history Capture() = %q, want nil", string(got))
	}
	if history.buf != nil {
		t.Fatal("released history buffer should be nil")
	}
	if history.capacity != 0 {
		t.Fatalf("released history capacity = %d, want 0", history.capacity)
	}
}

func TestKillPaneReleasesOutputHistoryForRemovedAndOrphanedPanes(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	visibleHistory := seededPaneOutputHistory("visible")
	orphanHistory := seededPaneOutputHistory("orphan")
	pane.OutputHistory = visibleHistory
	orphan := &TmuxPane{
		ID:            999,
		idString:      "%999",
		Window:        session.Windows[0],
		Env:           map[string]string{},
		OutputHistory: orphanHistory,
	}

	manager.mu.Lock()
	manager.panes[orphan.ID] = orphan
	manager.mu.Unlock()

	_, removedSession, err := manager.KillPane(pane.IDString())
	if err != nil {
		t.Fatalf("KillPane() error = %v", err)
	}
	if !removedSession {
		t.Fatal("removedSession = false, want true")
	}

	assertPaneHistoryReleased(t, pane, visibleHistory)
	assertPaneHistoryReleased(t, orphan, orphanHistory)
}

func TestRemoveSessionReleasesOutputHistoryForTrackedAndOrphanedPanes(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	secondPane, err := manager.SplitPane(pane.ID, SplitHorizontal)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}

	firstHistory := seededPaneOutputHistory("first")
	secondHistory := seededPaneOutputHistory("second")
	orphanHistory := seededPaneOutputHistory("orphan")
	pane.OutputHistory = firstHistory
	secondPane.OutputHistory = secondHistory
	orphan := &TmuxPane{
		ID:            999,
		idString:      "%999",
		Window:        session.Windows[0],
		Env:           map[string]string{},
		OutputHistory: orphanHistory,
	}

	manager.mu.Lock()
	manager.panes[orphan.ID] = orphan
	manager.mu.Unlock()

	if _, err := manager.RemoveSession("demo"); err != nil {
		t.Fatalf("RemoveSession() error = %v", err)
	}

	assertPaneHistoryReleased(t, pane, firstHistory)
	assertPaneHistoryReleased(t, secondPane, secondHistory)
	assertPaneHistoryReleased(t, orphan, orphanHistory)
}

func TestRemoveWindowByIDReleasesOutputHistoryForTrackedAndOrphanedPanes(t *testing.T) {
	manager := NewSessionManager()
	session, pane, err := manager.CreateSession("demo", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	windowID := session.Windows[0].ID
	visibleHistory := seededPaneOutputHistory("visible")
	orphanHistory := seededPaneOutputHistory("orphan")
	pane.OutputHistory = visibleHistory
	orphan := &TmuxPane{
		ID:            999,
		idString:      "%999",
		Window:        session.Windows[0],
		Env:           map[string]string{},
		OutputHistory: orphanHistory,
	}

	manager.mu.Lock()
	manager.panes[orphan.ID] = orphan
	manager.mu.Unlock()

	result, err := manager.RemoveWindowByID("demo", windowID)
	if err != nil {
		t.Fatalf("RemoveWindowByID() error = %v", err)
	}
	if !result.SessionRemoved {
		t.Fatal("SessionRemoved = false, want true")
	}
	if len(result.RemovedPanes) != 2 {
		t.Fatalf("len(RemovedPanes) = %d, want 2", len(result.RemovedPanes))
	}

	assertPaneHistoryReleased(t, pane, visibleHistory)
	assertPaneHistoryReleased(t, orphan, orphanHistory)
}

func TestCloseReleasesOutputHistory(t *testing.T) {
	manager := NewSessionManager()
	_, firstPane, err := manager.CreateSession("demo-a", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(demo-a) error = %v", err)
	}
	_, secondPane, err := manager.CreateSession("demo-b", "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession(demo-b) error = %v", err)
	}

	firstHistory := seededPaneOutputHistory("first")
	secondHistory := seededPaneOutputHistory("second")
	firstPane.OutputHistory = firstHistory
	secondPane.OutputHistory = secondHistory

	manager.Close()

	assertPaneHistoryReleased(t, firstPane, firstHistory)
	assertPaneHistoryReleased(t, secondPane, secondHistory)
}
