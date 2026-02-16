//go:build windows

package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strings"

	"myT-x/internal/userutil"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning is returned by TryLock when another instance holds the mutex.
var ErrAlreadyRunning = errors.New("another instance is already running")

// Lock holds a Windows named mutex handle for single-instance enforcement.
// The kernel automatically releases the mutex when the owning process terminates.
type Lock struct {
	handle windows.Handle
}

// TryLock attempts to acquire a system-wide named mutex.
// Returns ErrAlreadyRunning if another process already holds the mutex.
func TryLock(name string) (*Lock, error) {
	if name == "" {
		return nil, errors.New("mutex name is required")
	}
	nameUTF16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid mutex name %q: %w", name, err)
	}
	h, err := windows.CreateMutex(nil, true, nameUTF16)
	if err == windows.ERROR_ALREADY_EXISTS {
		// Another instance owns the mutex. Close the duplicate handle.
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("CreateMutex %q: %w", name, err)
	}
	return &Lock{handle: h}, nil
}

// Release closes the mutex handle. Safe to call on nil receiver and idempotent.
func (l *Lock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}

// sanitizeUsername replaces non-alphanumeric characters for use in mutex names.
// Delegates to userutil.SanitizeUsername for shared normalization behavior.
func sanitizeUsername(value string) string {
	return userutil.SanitizeUsername(value)
}

// DefaultMutexName returns the named mutex identifier for single-instance
// enforcement. The name mirrors the pipe naming convention from ipc.DefaultPipeName().
func DefaultMutexName() string {
	username := strings.TrimSpace(os.Getenv("USERNAME"))
	if username == "" {
		if current, err := user.Current(); err == nil {
			username = current.Username
		}
	}
	return `Global\myT-x-` + sanitizeUsername(username)
}
