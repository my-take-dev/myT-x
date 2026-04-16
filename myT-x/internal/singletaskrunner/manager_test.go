package singletaskrunner

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewServiceManagerPanicsOnNilFactory(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil factory")
		}
	}()

	NewServiceManager(nil)
}

func TestServiceManagerGetOrCreateAndRemove(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	first := manager.GetOrCreate("session-a")
	second := manager.GetOrCreate("session-a")
	other := manager.GetOrCreate("session-b")
	if first != second {
		t.Fatal("expected the same service for the same session")
	}
	if first == other {
		t.Fatal("expected different services for different sessions")
	}

	manager.Remove("session-a")
	recreated := manager.GetOrCreate("session-a")
	if recreated == first {
		t.Fatal("expected Remove to discard the previous session service")
	}
}

func TestServiceManagerGetStatusAndClearDelayWithoutService(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	status := manager.GetStatus("session-missing")
	if status.RunStatus != QueueIdle {
		t.Fatalf("RunStatus = %q, want %q", status.RunStatus, QueueIdle)
	}
	if status.CurrentIndex != -1 {
		t.Fatalf("CurrentIndex = %d, want -1", status.CurrentIndex)
	}
	if status.SessionName != "session-missing" {
		t.Fatalf("SessionName = %q, want %q", status.SessionName, "session-missing")
	}
	if len(status.Items) != 0 {
		t.Fatalf("Items len = %d, want 0", len(status.Items))
	}
	if got := manager.GetClearDelay("session-missing"); got != DefaultClearDelay {
		t.Fatalf("GetClearDelay() = %d, want %d", got, DefaultClearDelay)
	}
}

func TestServiceManagerStopAllStopsActiveServices(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	sessionNames := []string{"session-a", "session-b"}
	for _, sessionName := range sessionNames {
		svc := manager.GetOrCreate(sessionName)
		if err := svc.SetClearDelay(0); err != nil {
			t.Fatalf("SetClearDelay(%q): %v", sessionName, err)
		}
		if err := svc.AddItem("Task", "message", "%1", false, ""); err != nil {
			t.Fatalf("AddItem(%q): %v", sessionName, err)
		}
		if err := svc.Start(); err != nil {
			t.Fatalf("Start(%q): %v", sessionName, err)
		}
	}

	manager.StopAll()

	waitForCondition(t, time.Second, "all services stopped", func() bool {
		for _, sessionName := range sessionNames {
			status := manager.GetStatus(sessionName)
			if status.RunStatus != QueueIdle {
				return false
			}
		}
		return true
	})
}

func TestServiceManagerRemoveStopsRunningService(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	svc := manager.GetOrCreate("session-a")
	if err := svc.SetClearDelay(0); err != nil {
		t.Fatalf("SetClearDelay: %v", err)
	}
	if err := svc.AddItem("Task", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if err := svc.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	manager.Remove("session-a")

	waitForCondition(t, time.Second, "service stopped after remove", func() bool {
		return svc.GetStatus().RunStatus == QueueIdle
	})

	recreated := manager.GetOrCreate("session-a")
	if recreated == svc {
		t.Fatal("expected Remove to discard the previous running service")
	}
	if got := manager.GetClearDelay("session-a"); got != DefaultClearDelay {
		t.Fatalf("GetClearDelay() after recreate = %d, want %d", got, DefaultClearDelay)
	}
}

func TestServiceManagerRemoveMissingSessionIsNoop(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	manager.Remove("missing")

	status := manager.GetStatus("missing")
	if status.SessionName != "missing" {
		t.Fatalf("SessionName = %q, want %q", status.SessionName, "missing")
	}
	if got := manager.GetClearDelay("missing"); got != DefaultClearDelay {
		t.Fatalf("GetClearDelay() = %d, want %d", got, DefaultClearDelay)
	}
}

func TestServiceManagerGetStatusCopiesItems(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	svc := manager.GetOrCreate("session-a")
	if err := svc.AddItem("Task", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	status := manager.GetStatus("session-a")
	status.Items[0].Title = "mutated"

	fresh := manager.GetStatus("session-a")
	if strings.EqualFold(fresh.Items[0].Title, "mutated") {
		t.Fatal("GetStatus should return a copy of queue items")
	}
}

func TestServiceManagerRenameMigratesService(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	svc := manager.GetOrCreate("old-session")
	if err := svc.AddItem("Task", "message", "%1", false, ""); err != nil {
		t.Fatalf("AddItem: %v", err)
	}

	if err := manager.Rename("old-session", "new-session"); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if got := manager.GetStatus("new-session"); got.SessionName != "new-session" || len(got.Items) != 1 {
		t.Fatalf("GetStatus(new-session) = %+v, want migrated service state", got)
	}
	if got := manager.GetStatus("old-session"); got.SessionName != "old-session" || len(got.Items) != 0 {
		t.Fatalf("GetStatus(old-session) = %+v, want empty status after rename", got)
	}
}

func TestServiceManagerRenameRejectsExistingTarget(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})

	_ = manager.GetOrCreate("old-session")
	_ = manager.GetOrCreate("new-session")

	if err := manager.Rename("old-session", "new-session"); err == nil {
		t.Fatal("Rename() expected conflict error when target session already exists")
	}
}

func TestServiceManagerRenameKeepsOriginalMappingWhenServiceRenameFails(t *testing.T) {
	t.Parallel()

	manager := NewServiceManager(func(sessionName string) Deps {
		deps := testDeps(nil, nil)
		deps.SessionName = sessionName
		return deps
	})
	svc := manager.GetOrCreate("old-session")
	manager.rename = func(_ *Service, sessionName string) error {
		if sessionName != "new-session" {
			t.Fatalf("rename target = %q, want %q", sessionName, "new-session")
		}
		return errors.New("rename failed")
	}

	err := manager.Rename("old-session", "new-session")
	if err == nil {
		t.Fatal("Rename() expected error when service rename fails")
	}
	if got := manager.GetStatus("old-session"); got.SessionName != "old-session" {
		t.Fatalf("GetStatus(old-session).SessionName = %q, want %q", got.SessionName, "old-session")
	}
	if got := manager.GetStatus("new-session"); got.SessionName != "new-session" || len(got.Items) != 0 {
		t.Fatalf("GetStatus(new-session) = %+v, want empty status for untouched target", got)
	}
	if svc.GetStatus().SessionName != "old-session" {
		t.Fatalf("service session name = %q, want %q", svc.GetStatus().SessionName, "old-session")
	}
}
