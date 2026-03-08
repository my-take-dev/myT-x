package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"myT-x/internal/ipc"
)

const defaultMCPStdioDialTimeout = 3 * time.Second

type mcpStdioCLIConfig struct {
	sessionName string
	mcpName     string
	dialTimeout time.Duration
}

var sendIPCRequestFn = ipc.Send
var mcpStdioPlatformSupportedFn = func() bool { return runtime.GOOS == "windows" }

// mcpCLIPipeNameFn is a test seam for overriding the default pipe name in unit tests.
var mcpCLIPipeNameFn = ipc.DefaultPipeName

func runMCPCLIMode(args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	if !strings.EqualFold(strings.TrimSpace(args[0]), "mcp") {
		return false, 0
	}
	return true, executeMCPCLI(args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func executeMCPCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printMCPCLIUsage(stderr)
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "stdio":
		cfg, err := parseMCPStdioCLI(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "mcp stdio: %v\n", err)
			printMCPCLIUsage(stderr)
			return 2
		}
		if !mcpStdioPlatformSupportedFn() {
			fmt.Fprintln(stderr, "mcp stdio: mcp stdio mode is supported only on Windows")
			return 1
		}
		resolved, err := resolveMCPStdioViaIPC(cfg.sessionName, cfg.mcpName)
		if err != nil {
			fmt.Fprintf(stderr, "mcp stdio: %v\n", err)
			return 1
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := bridgeMCPStdio(ctx, resolved.PipePath, cfg.dialTimeout, stdin, stdout); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(stderr, "mcp stdio: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown mcp subcommand: %s\n", args[0])
		printMCPCLIUsage(stderr)
		return 2
	}
}

func parseMCPStdioCLI(args []string) (mcpStdioCLIConfig, error) {
	fs := flag.NewFlagSet("mcp stdio", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionName := fs.String("session", "", "myT-x session name")
	mcpName := fs.String("mcp", "", "MCP id or alias (for example: gopls)")
	dialTimeoutMS := fs.Int("dial-timeout-ms", int(defaultMCPStdioDialTimeout/time.Millisecond), "Pipe dial timeout in milliseconds")

	if err := fs.Parse(args); err != nil {
		return mcpStdioCLIConfig{}, err
	}

	session := strings.TrimSpace(*sessionName)
	if session == "" {
		return mcpStdioCLIConfig{}, errors.New("--session is required")
	}
	mcpTarget := strings.TrimSpace(*mcpName)
	if mcpTarget == "" {
		return mcpStdioCLIConfig{}, errors.New("--mcp is required")
	}
	if *dialTimeoutMS <= 0 {
		return mcpStdioCLIConfig{}, errors.New("--dial-timeout-ms must be > 0")
	}

	return mcpStdioCLIConfig{
		sessionName: session,
		mcpName:     mcpTarget,
		dialTimeout: time.Duration(*dialTimeoutMS) * time.Millisecond,
	}, nil
}

func resolveMCPStdioViaIPC(sessionName, mcpName string) (ipc.MCPStdioResolvePayload, error) {
	resp, err := sendIPCRequestFn(mcpCLIPipeNameFn(), ipc.TmuxRequest{
		Command: "mcp-resolve-stdio",
		Flags: map[string]any{
			"session": sessionName,
			"mcp":     mcpName,
		},
	})
	if err != nil {
		if ipc.IsConnectionError(err) {
			return ipc.MCPStdioResolvePayload{}, fmt.Errorf("myT-x IPC is unavailable. Start myT-x GUI first")
		}
		return ipc.MCPStdioResolvePayload{}, fmt.Errorf("ipc request failed: %w", err)
	}
	if resp.ExitCode != 0 {
		message := strings.TrimSpace(resp.Stderr)
		if message == "" {
			message = "unknown error"
		}
		return ipc.MCPStdioResolvePayload{}, errors.New(message)
	}

	var resolved ipc.MCPStdioResolvePayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Stdout)), &resolved); err != nil {
		return ipc.MCPStdioResolvePayload{}, fmt.Errorf("parse ipc payload: %w", err)
	}
	if strings.TrimSpace(resolved.SessionName) == "" || strings.TrimSpace(resolved.MCPID) == "" || strings.TrimSpace(resolved.PipePath) == "" {
		return ipc.MCPStdioResolvePayload{}, errors.New("ipc payload is missing session_name, mcp_id, or pipe_path")
	}
	// Safety net: reject a mismatched session_name so the CLI never bridges to
	// a different session if the IPC server returns the wrong payload.
	if !sameResolvedSessionName(sessionName, resolved.SessionName) {
		return ipc.MCPStdioResolvePayload{}, fmt.Errorf(
			"ipc payload session_name = %q, want %q",
			resolved.SessionName,
			sessionName,
		)
	}
	return resolved, nil
}

func sameResolvedSessionName(expected, actual string) bool {
	return strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(actual))
}

func printMCPCLIUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  myT-x mcp stdio --session <session-name> --mcp <mcp-name>")
}
