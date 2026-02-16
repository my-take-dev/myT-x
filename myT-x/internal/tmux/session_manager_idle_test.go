package tmux

import (
	"testing"
	"time"
)

func TestRecommendedIdleCheckInterval(t *testing.T) {
	t.Run("returns slow interval when no sessions", func(t *testing.T) {
		manager := NewSessionManager()
		if got := manager.RecommendedIdleCheckInterval(); got != 5*time.Second {
			t.Fatalf("RecommendedIdleCheckInterval() = %v, want %v", got, 5*time.Second)
		}
	})

	t.Run("returns fast interval when any session is active", func(t *testing.T) {
		manager := NewSessionManager()
		session, _, err := manager.CreateSession("demo", "0", 120, 40)
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		manager.mu.Lock()
		session.IsIdle = false
		manager.mu.Unlock()

		if got := manager.RecommendedIdleCheckInterval(); got != time.Second {
			t.Fatalf("RecommendedIdleCheckInterval() = %v, want %v", got, time.Second)
		}
	})

	t.Run("returns slow interval when all sessions are idle", func(t *testing.T) {
		manager := NewSessionManager()
		if _, _, err := manager.CreateSession("alpha", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession(alpha) error = %v", err)
		}
		if _, _, err := manager.CreateSession("beta", "0", 120, 40); err != nil {
			t.Fatalf("CreateSession(beta) error = %v", err)
		}

		manager.mu.Lock()
		for _, session := range manager.sessions {
			session.IsIdle = true
		}
		manager.mu.Unlock()

		if got := manager.RecommendedIdleCheckInterval(); got != 5*time.Second {
			t.Fatalf("RecommendedIdleCheckInterval() = %v, want %v", got, 5*time.Second)
		}
	})

	t.Run("ignores nil session entries", func(t *testing.T) {
		manager := NewSessionManager()
		manager.mu.Lock()
		manager.sessions["broken"] = nil
		manager.mu.Unlock()

		if got := manager.RecommendedIdleCheckInterval(); got != 5*time.Second {
			t.Fatalf("RecommendedIdleCheckInterval() = %v, want %v", got, 5*time.Second)
		}
	})
}
