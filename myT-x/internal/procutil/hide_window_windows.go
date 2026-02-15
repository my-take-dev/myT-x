//go:build windows

package procutil

import (
	"os/exec"
	"syscall"
)

// HideWindow configures cmd to suppress the console window flash on Windows.
// Preserves any existing SysProcAttr fields that were set before this call.
func HideWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
