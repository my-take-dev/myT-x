//go:build windows

package terminal

import "os"

// resizePtmx is a stub on Windows. ptmx is never set when using ConPTY.
func resizePtmx(_ *os.File, _, _ int) error {
	return nil
}
