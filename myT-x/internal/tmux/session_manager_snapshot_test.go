package tmux

import (
	"reflect"
	"testing"
)

func TestSortedSessionNamesLockedSortsBySessionID(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("first", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(first) error = %v", err)
	}
	if _, _, err := manager.CreateSession("second", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(second) error = %v", err)
	}
	if _, _, err := manager.CreateSession("third", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(third) error = %v", err)
	}

	manager.mu.Lock()
	names := manager.sortedSessionNamesLocked()
	manager.mu.Unlock()

	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("sortedSessionNamesLocked() = %#v, want %#v", names, want)
	}
}

func TestSortedSessionNamesLockedReusesCacheWhenClean(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("alpha", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(alpha) error = %v", err)
	}
	if _, _, err := manager.CreateSession("beta", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(beta) error = %v", err)
	}

	manager.mu.Lock()
	first := manager.sortedSessionNamesLocked()
	second := manager.sortedSessionNamesLocked()
	manager.mu.Unlock()

	if len(first) != len(second) {
		t.Fatalf("cache length mismatch: first=%d second=%d", len(first), len(second))
	}
	if len(first) > 0 && &first[0] != &second[0] {
		t.Fatal("sortedSessionNamesLocked() should reuse cached slice when not dirty")
	}
}

func TestSortedSessionNamesLockedRebuildsAfterSessionMapMutation(t *testing.T) {
	manager := NewSessionManager()
	if _, _, err := manager.CreateSession("alpha", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(alpha) error = %v", err)
	}
	if _, _, err := manager.CreateSession("beta", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(beta) error = %v", err)
	}

	manager.mu.Lock()
	before := append([]string(nil), manager.sortedSessionNamesLocked()...)
	manager.mu.Unlock()

	if _, _, err := manager.CreateSession("gamma", "0", 120, 40); err != nil {
		t.Fatalf("CreateSession(gamma) error = %v", err)
	}

	manager.mu.Lock()
	after := manager.sortedSessionNamesLocked()
	manager.mu.Unlock()

	if len(after) != len(before)+1 {
		t.Fatalf("sortedSessionNamesLocked() length = %d, want %d", len(after), len(before)+1)
	}
	if after[len(after)-1] != "gamma" {
		t.Fatalf("last session = %q, want %q", after[len(after)-1], "gamma")
	}
}
