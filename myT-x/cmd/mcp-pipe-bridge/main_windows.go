//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"

	"myT-x/internal/mcp/pipebridge"
)

const defaultDialTimeout = 5 * time.Second

type cliConfig struct {
	pipeName    string
	dialTimeout time.Duration
}

func main() {
	cfg, err := parseCLI(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-pipe-bridge: %v\n", err)
		os.Exit(2)
	}

	dialTimeout := cfg.dialTimeout
	conn, err := winio.DialPipe(cfg.pipeName, &dialTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-pipe-bridge: dial pipe %q failed: %v\n", cfg.pipeName, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := pipebridge.Bridge(ctx, os.Stdin, os.Stdout, conn); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "mcp-pipe-bridge: relay failed: %v\n", err)
		os.Exit(1)
	}
}

func parseCLI(args []string) (cliConfig, error) {
	fs := flag.NewFlagSet("mcp-pipe-bridge", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	pipeName := fs.String("pipe", "", "Named Pipe path (\\\\.\\pipe\\...)")
	dialTimeoutMS := fs.Int("dial-timeout-ms", int(defaultDialTimeout/time.Millisecond), "Dial timeout in milliseconds")
	if err := fs.Parse(args); err != nil {
		return cliConfig{}, err
	}

	resolvedPipeName := strings.TrimSpace(*pipeName)
	if resolvedPipeName == "" {
		resolvedPipeName = strings.TrimSpace(os.Getenv("MYTX_MCP_PIPE"))
	}
	if resolvedPipeName == "" {
		return cliConfig{}, errors.New("-pipe is required (or set MYTX_MCP_PIPE)")
	}
	if !strings.HasPrefix(resolvedPipeName, `\\.\pipe\`) {
		return cliConfig{}, fmt.Errorf("invalid pipe path %q", resolvedPipeName)
	}
	if *dialTimeoutMS <= 0 {
		return cliConfig{}, errors.New("-dial-timeout-ms must be > 0")
	}

	return cliConfig{
		pipeName:    resolvedPipeName,
		dialTimeout: time.Duration(*dialTimeoutMS) * time.Millisecond,
	}, nil
}
