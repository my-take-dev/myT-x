//go:build !windows

package lsp

import "os/exec"

func testHideWindowEnabled(*exec.Cmd) bool {
	return false
}
