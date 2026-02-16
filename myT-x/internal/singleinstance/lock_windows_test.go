//go:build windows

package singleinstance

import (
	"strings"
	"testing"
)

func TestTryLock(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "first lock succeeds",
			run: func(t *testing.T) {
				lock, err := TryLock(`Global\myT-x-test-first`)
				if err != nil {
					t.Fatalf("TryLock failed: %v", err)
				}
				if lock == nil {
					t.Fatal("TryLock returned nil lock without error")
				}
				if err := lock.Release(); err != nil {
					t.Fatalf("Release failed: %v", err)
				}
			},
		},
		{
			name: "second lock returns ErrAlreadyRunning",
			run: func(t *testing.T) {
				lock1, err := TryLock(`Global\myT-x-test-second`)
				if err != nil {
					t.Fatalf("first TryLock failed: %v", err)
				}
				defer lock1.Release()

				lock2, err := TryLock(`Global\myT-x-test-second`)
				if err != ErrAlreadyRunning {
					t.Fatalf("second TryLock: got err=%v, want ErrAlreadyRunning", err)
				}
				if lock2 != nil {
					t.Fatal("second TryLock returned non-nil lock on ErrAlreadyRunning")
				}
			},
		},
		{
			name: "lock reacquirable after release",
			run: func(t *testing.T) {
				lock1, err := TryLock(`Global\myT-x-test-reacquire`)
				if err != nil {
					t.Fatalf("first TryLock failed: %v", err)
				}
				if err := lock1.Release(); err != nil {
					t.Fatalf("Release failed: %v", err)
				}

				lock2, err := TryLock(`Global\myT-x-test-reacquire`)
				if err != nil {
					t.Fatalf("second TryLock after release failed: %v", err)
				}
				defer lock2.Release()
			},
		},
		{
			name: "release idempotent",
			run: func(t *testing.T) {
				lock, err := TryLock(`Global\myT-x-test-idempotent`)
				if err != nil {
					t.Fatalf("TryLock failed: %v", err)
				}
				if err := lock.Release(); err != nil {
					t.Fatalf("first Release failed: %v", err)
				}
				if err := lock.Release(); err != nil {
					t.Fatalf("second Release should be no-op, got: %v", err)
				}
			},
		},
		{
			name: "nil lock release safe",
			run: func(t *testing.T) {
				var lock *Lock
				if err := lock.Release(); err != nil {
					t.Fatalf("nil Release should be no-op, got: %v", err)
				}
			},
		},
		{
			name: "empty name returns error",
			run: func(t *testing.T) {
				lock, err := TryLock("")
				if err == nil {
					t.Fatal("TryLock with empty name should fail")
				}
				if lock != nil {
					lock.Release()
					t.Fatal("TryLock with empty name returned non-nil lock")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestDefaultMutexName(t *testing.T) {
	name := DefaultMutexName()
	if name == "" {
		t.Fatal("DefaultMutexName returned empty string")
	}
	if !strings.HasPrefix(name, `Global\myT-x-`) {
		t.Fatalf("DefaultMutexName = %q, want prefix %q", name, `Global\myT-x-`)
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"alice", "alice"},
		{"DOMAIN\\user", "DOMAIN_user"},
		{"user@domain.com", "user_domain.com"},
		{"", "unknown"},
		{"  ", "unknown"},
	}
	for _, tt := range tests {
		got := sanitizeUsername(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeUsername(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
