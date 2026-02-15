//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestHideWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "echo", "test")

	HideWindow(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil after HideWindow()")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is false, want true")
	}
}

func TestHideWindowPreservesExistingSysProcAttr(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "echo", "test")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	HideWindow(cmd)

	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow is false, want true")
	}
	if cmd.SysProcAttr.CreationFlags != syscall.CREATE_NEW_PROCESS_GROUP {
		t.Errorf("CreationFlags = %d, want %d", cmd.SysProcAttr.CreationFlags, syscall.CREATE_NEW_PROCESS_GROUP)
	}
}

func TestHideWindowIdempotent(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "echo", "test")
	HideWindow(cmd)
	HideWindow(cmd) // double call must not panic or change state

	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow should remain true after double call")
	}
}

func TestHideWindowNilCmd(t *testing.T) {
	// Must not panic on nil cmd.
	HideWindow(nil)
}
