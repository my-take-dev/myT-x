package taskscheduler

import (
	"testing"
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
	m.Remove("ses-1")

	// After remove, GetOrCreate should create a new instance.
	svc2 := m.GetOrCreate("ses-1")
	if svc1 == svc2 {
		t.Error("after Remove, GetOrCreate should create a new instance")
	}

	// Remove nonexistent session should not panic.
	m.Remove("nonexistent")
}
