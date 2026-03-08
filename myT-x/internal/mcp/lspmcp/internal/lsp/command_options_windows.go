//go:build windows

package lsp

import (
	"os/exec"
	"syscall"
)

// applyPlatformExecOptions sets Windows-specific process options for child LSP
// processes so no extra console window is shown when myT-x starts an MCP.
func applyPlatformExecOptions(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
