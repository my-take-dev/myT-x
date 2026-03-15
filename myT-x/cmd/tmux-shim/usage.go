package main

import (
	"fmt"
	"io"
	"os"
)

func printUsage() {
	// NOTE: Usage output is best-effort; write failures are non-fatal for the shim.
	renderUsage(os.Stdout)
}

func renderUsage(w io.Writer) {
	const commandPadding = 18

	_, _ = fmt.Fprintln(w, "tmux shim for myT-x")
	_, _ = fmt.Fprintln(w, "Usage: tmux <command> [flags] [args]")
	_, _ = fmt.Fprintln(w, "Supported commands:")
	for _, name := range commandOrder {
		description := commandSpecs[name].description
		_, _ = fmt.Fprintf(w, "  %-*s  %s\n", commandPadding, name, description)
	}
}
