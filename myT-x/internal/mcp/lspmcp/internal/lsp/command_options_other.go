//go:build !windows

package lsp

import "os/exec"

func applyPlatformExecOptions(*exec.Cmd) {}
