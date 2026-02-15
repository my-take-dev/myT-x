package main

import "fmt"

func printUsage() {
	// NOTE: Usage output is best-effort; write failures are non-fatal for the shim.
	_, _ = fmt.Println("tmux shim for myT-x")
	_, _ = fmt.Println("Usage: tmux <command> [flags] [args]")
	_, _ = fmt.Println("Supported commands:")
	for _, name := range commandOrder {
		_, _ = fmt.Printf("  %s\n", name)
	}
}
