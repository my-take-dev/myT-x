//go:build !windows

package main

import "errors"

// openExplorer is not supported on non-Windows platforms.
func openExplorer(_ string) error {
	return errors.New("openExplorer is not supported on this platform")
}
