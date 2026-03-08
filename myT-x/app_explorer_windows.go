//go:build windows

package main

import (
	"os/exec"
)

// openExplorer launches Windows Explorer at the specified directory path.
func openExplorer(dirPath string) error {
	cmd := exec.Command("explorer.exe", dirPath)
	return cmd.Start()
}
