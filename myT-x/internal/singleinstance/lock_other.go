//go:build !windows

package singleinstance

import "errors"

// ErrAlreadyRunning is returned by TryLock when another instance holds the mutex.
var ErrAlreadyRunning = errors.New("another instance is already running")

// Lock is a no-op on non-Windows platforms.
type Lock struct{}

// TryLock always succeeds on non-Windows platforms.
func TryLock(_ string) (*Lock, error) { return &Lock{}, nil }

// Release is a no-op on non-Windows platforms.
func (l *Lock) Release() error { return nil }

// DefaultMutexName returns an empty string on non-Windows platforms.
func DefaultMutexName() string { return "" }
