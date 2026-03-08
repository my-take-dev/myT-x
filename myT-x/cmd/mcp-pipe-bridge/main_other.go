//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "mcp-pipe-bridge is supported only on Windows")
	os.Exit(1)
}
