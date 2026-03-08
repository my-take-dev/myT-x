//go:build windows

package lsp

import "os/exec"

func testHideWindowEnabled(cmd *exec.Cmd) bool {
	if cmd == nil || cmd.SysProcAttr == nil {
		return false
	}
	return cmd.SysProcAttr.HideWindow
}
