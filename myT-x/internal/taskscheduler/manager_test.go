package taskscheduler

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestNewServiceManager_PanicsOnNilFactory(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil factory, got none")
		}
	}()
	NewServiceManager(nil)
}

func TestServiceManager_GetOrCreate(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	svc1 := m.GetOrCreate("session-a")
	svc2 := m.GetOrCreate("session-a")
	svc3 := m.GetOrCreate("session-b")

	if svc1 != svc2 {
		t.Error("GetOrCreate should return the same instance for the same session")
	}
	if svc1 == svc3 {
		t.Error("GetOrCreate should return different instances for different sessions")
	}
}

func TestServiceManager_GetStatus_NoService(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	status := m.GetStatus("nonexistent")
	if status.RunStatus != QueueIdle {
		t.Errorf("expected idle, got %q", status.RunStatus)
	}
	if status.CurrentIndex != -1 {
		t.Errorf("expected -1, got %d", status.CurrentIndex)
	}
	if status.Items == nil {
		t.Fatal("expected non-nil items")
	}
	if len(status.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(status.Items))
	}
	if status.SessionName != "nonexistent" {
		t.Errorf("expected session_name=%q, got %q", "nonexistent", status.SessionName)
	}
}

func TestServiceManager_GetStatus_WithService(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	svc := m.GetOrCreate("ses-1")
	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	status := m.GetStatus("ses-1")
	if len(status.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(status.Items))
	}
	if status.SessionName != "ses-1" {
		t.Errorf("expected session_name=%q, got %q", "ses-1", status.SessionName)
	}
	if status.GenerationID == "" {
		t.Fatal("expected generation_id to be populated")
	}
}

func TestServiceManager_StopAll(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	_ = m.GetOrCreate("ses-1")
	_ = m.GetOrCreate("ses-2")

	// Should not panic.
	m.StopAll()
	m.StopAll()
}

func TestServiceManager_Remove(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	svc1 := m.GetOrCreate("ses-1")
	originalGenerationID := svc1.GetStatus().GenerationID
	m.Remove("ses-1")

	// After remove, GetOrCreate should create a new instance.
	svc2 := m.GetOrCreate("ses-1")
	if svc1 == svc2 {
		t.Error("after Remove, GetOrCreate should create a new instance")
	}
	if svc2.GetStatus().GenerationID == "" {
		t.Fatal("after Remove, recreated service should use a non-empty generation id")
	}
	if originalGenerationID == svc2.GetStatus().GenerationID {
		t.Error("after Remove, recreated service should use a new generation id")
	}

	// Remove nonexistent session should not panic.
	m.Remove("nonexistent")
}

func TestServiceManager_RemoveRetiresOldService(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	svc := m.GetOrCreate("ses-1")
	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	m.Remove("ses-1")

	// The removed service should be retired: GetStatus returns default snapshot.
	status := svc.GetStatus()
	if len(status.Items) != 0 {
		t.Fatalf("retired service Items = %d, want 0", len(status.Items))
	}
	if status.RunStatus != QueueIdle {
		t.Fatalf("retired service RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}

	// Public mutation methods should be rejected.
	if err := svc.AddItem("task2", "msg", "%0", false, ""); !errors.Is(err, errServiceRetired) {
		t.Fatalf("AddItem on retired service: error = %v, want %v", err, errServiceRetired)
	}
}

func TestServiceManager_RenameMigratesService(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	svc := m.GetOrCreate("old-session")
	if err := svc.AddItem("task1", "msg", "%0", false, ""); err != nil {
		t.Fatal(err)
	}

	if err := m.Rename("old-session", "new-session"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if got := m.GetStatus("new-session"); got.SessionName != "new-session" || len(got.Items) != 1 {
		t.Fatalf("GetStatus(new-session) = %+v, want migrated service state", got)
	}
	if got := m.GetStatus("old-session"); got.SessionName != "old-session" || len(got.Items) != 0 {
		t.Fatalf("GetStatus(old-session) = %+v, want empty status after rename", got)
	}
}

func TestServiceManager_RenameWhileRunningKeepsCurrentDBBindingAndRebindsNextRun(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	oldDBPath := filepath.Join(tempDir, "old-session.db")
	newDBPath := filepath.Join(tempDir, "new-session.db")
	prepareTaskSchedulerDB(t, oldDBPath)
	prepareTaskSchedulerDB(t, newDBPath)

	sendStarted := make(chan struct{})
	releaseSend := make(chan struct{})

	var (
		mu               sync.Mutex
		resolvedSessions []string
	)

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		deps.ResolveOrchestratorDBPath = func(boundSessionName string) (string, error) {
			mu.Lock()
			resolvedSessions = append(resolvedSessions, boundSessionName)
			mu.Unlock()

			switch boundSessionName {
			case "old-session":
				return oldDBPath, nil
			case "new-session":
				return newDBPath, nil
			default:
				return "", fmt.Errorf("unexpected session %q", boundSessionName)
			}
		}
		deps.SendMessagePaste = func(string, string) error {
			select {
			case <-sendStarted:
			default:
				close(sendStarted)
			}
			<-releaseSend
			return errors.New("paste failed")
		}
		return deps
	})

	svc := m.GetOrCreate("old-session")
	items := []QueueItem{{Title: "task1", Message: "msg", TargetPaneID: "%0"}}
	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("Start(old-session): %v", err)
	}

	select {
	case <-sendStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SendMessagePaste to start")
	}

	if err := m.Rename("old-session", "new-session"); err != nil {
		t.Fatalf("Rename(): %v", err)
	}
	if got := m.GetStatus("new-session").SessionName; got != "new-session" {
		t.Fatalf("GetStatus(new-session).SessionName = %q, want %q", got, "new-session")
	}

	close(releaseSend)

	waitForCondition(t, 2*time.Second, "first run to stop after send failure", func() bool {
		return m.GetStatus("new-session").RunStatus == QueueIdle
	})

	if err := svc.Start(QueueConfig{}, items); err != nil {
		t.Fatalf("Start(new-session): %v", err)
	}

	waitForCondition(t, 2*time.Second, "second run db bind", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(resolvedSessions) >= 2
	})

	mu.Lock()
	got := slices.Clone(resolvedSessions)
	mu.Unlock()
	if len(got) < 2 {
		t.Fatalf("resolved sessions = %v, want at least two binds", got)
	}
	if got[0] != "old-session" {
		t.Fatalf("first db bind session = %q, want %q", got[0], "old-session")
	}
	if got[1] != "new-session" {
		t.Fatalf("second db bind session = %q, want %q", got[1], "new-session")
	}
}

func TestServiceManager_RenameRejectsExistingTarget(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})

	_ = m.GetOrCreate("old-session")
	_ = m.GetOrCreate("new-session")

	if err := m.Rename("old-session", "new-session"); err == nil {
		t.Fatal("Rename() expected conflict error when target session already exists")
	}
}

func TestServiceManager_RenameKeepsOriginalMappingWhenServiceRenameFails(t *testing.T) {
	t.Parallel()

	m := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps()
		deps.SessionName = sessionName
		return deps
	})
	svc := m.GetOrCreate("old-session")
	m.rename = func(_ *Service, sessionName string) error {
		if sessionName != "new-session" {
			t.Fatalf("rename target = %q, want %q", sessionName, "new-session")
		}
		return errors.New("rename failed")
	}

	err := m.Rename("old-session", "new-session")
	if err == nil {
		t.Fatal("Rename() expected error when service rename fails")
	}
	if got := m.GetStatus("old-session"); got.SessionName != "old-session" {
		t.Fatalf("GetStatus(old-session).SessionName = %q, want %q", got.SessionName, "old-session")
	}
	if got := m.GetStatus("new-session"); got.SessionName != "new-session" || len(got.Items) != 0 {
		t.Fatalf("GetStatus(new-session) = %+v, want empty status for untouched target", got)
	}
	if svc.GetStatus().SessionName != "old-session" {
		t.Fatalf("service session name = %q, want %q", svc.GetStatus().SessionName, "old-session")
	}
}
