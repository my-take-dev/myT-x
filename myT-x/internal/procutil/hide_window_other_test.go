//go:build !windows

package procutil

import (
	"os/exec"
	"testing"
)

func TestHideWindowNoOpOnNonWindows(t *testing.T) {
	cmd := exec.Command("echo", "test")
	HideWindow(cmd)

	// SysProcAttr should remain nil on non-Windows.
	if cmd.SysProcAttr != nil {
		t.Fatal("SysProcAttr should be nil on non-Windows after HideWindow")
	}
}

func TestHideWindowNilCmdNoOpOnNonWindows(t *testing.T) {
	// Must not panic.
	HideWindow(nil)
}
