package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"myT-x/internal/ipc"
)

const (
	defaultMCPStdioDialTimeout = 8 * time.Second

	ipcResolveMaxRetries    = 3
	ipcResolveRetryInterval = 500 * time.Millisecond
)

type mcpStdioCLIConfig struct {
	sessionName string
	mcpName     string
	dialTimeout time.Duration
}

// mcpCLIDeps holds injectable functions for MCP CLI operations.
// Tests create custom instances instead of mutating package-level state.
type mcpCLIDeps struct {
	// sendIPCRequest sends an IPC request to the named pipe and returns the response.
	sendIPCRequest func(string, ipc.TmuxRequest) (ipc.TmuxResponse, error)
	// platformSupported returns true if the current platform supports MCP stdio.
	platformSupported func() bool
	// pipeName returns the named pipe path for IPC communication.
	pipeName func() string
	// resolveSessionByEnv returns the session name from the MYTX_SESSION env var.
	resolveSessionByEnv func() string
	// resolveSessionByCwd resolves the session name via IPC using the given cwd.
	resolveSessionByCwd func(pipeName, cwd string) (string, error)
	// getwd returns the current working directory.
	getwd func() (string, error)
}

func defaultMCPCLIDeps() mcpCLIDeps {
	d := mcpCLIDeps{
		sendIPCRequest:    ipc.Send,
		platformSupported: func() bool { return runtime.GOOS == "windows" },
		pipeName:          ipc.DefaultPipeName,
		resolveSessionByEnv: func() string {
			return strings.TrimSpace(os.Getenv("MYTX_SESSION"))
		},
		getwd: os.Getwd,
	}
	d.resolveSessionByCwd = func(pipeName, cwd string) (string, error) {
		resp, err := d.sendIPCRequest(pipeName, ipc.TmuxRequest{
			Command: "resolve-session-by-cwd",
			Flags:   map[string]any{"cwd": cwd},
		})
		if err != nil {
			return "", fmt.Errorf("ipc request failed: %w", err)
		}
		if resp.ExitCode != 0 {
			msg := strings.TrimSpace(resp.Stderr)
			if msg == "" {
				msg = "session not found"
			}
			return "", errors.New(msg)
		}
		resolved := strings.TrimSpace(resp.Stdout)
		if resolved == "" {
			return "", errors.New("ipc returned empty session name")
		}
		return resolved, nil
	}
	return d
}

func runMCPCLIMode(args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	if !strings.EqualFold(strings.TrimSpace(args[0]), "mcp") {
		return false, 0
	}
	return true, defaultMCPCLIDeps().executeMCPCLIWith(args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func (d mcpCLIDeps) executeMCPCLIWith(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printMCPCLIUsage(stderr)
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "stdio":
		cfg, err := d.parseMCPStdioCLI(args[1:])
		if err != nil {
			fmt.Fprintf(stderr, "mcp stdio: %v\n", err)
			printMCPCLIUsage(stderr)
			return 2
		}
		if !d.platformSupported() {
			fmt.Fprintln(stderr, "mcp stdio: mcp stdio mode is supported only on Windows")
			return 1
		}
		resolved, err := d.resolveMCPStdioViaIPC(cfg.sessionName, cfg.mcpName)
		if err != nil {
			fmt.Fprintf(stderr, "mcp stdio: %v\n", err)
			return 1
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := bridgeMCPStdio(ctx, resolved.PipePath, cfg.dialTimeout, stdin, stdout, dialPipe); err != nil && !errors.Is(err, context.Canceled) {
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

func (d mcpCLIDeps) parseMCPStdioCLI(args []string) (mcpStdioCLIConfig, error) {
	fs := flag.NewFlagSet("mcp stdio", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	sessionName := fs.String("session", "", "myT-x session name")
	mcpName := fs.String("mcp", "", "MCP id or alias (for example: gopls)")
	dialTimeoutMS := fs.Int("dial-timeout-ms", int(defaultMCPStdioDialTimeout/time.Millisecond), "Pipe dial timeout in milliseconds")

	if err := fs.Parse(args); err != nil {
		return mcpStdioCLIConfig{}, err
	}

	// 3-stage fallback: --session flag > $MYTX_SESSION > IPC resolve-session-by-cwd
	session := strings.TrimSpace(*sessionName)
	if session == "" {
		session = d.resolveSessionByEnv()
	}
	if session == "" {
		cwd, cwdErr := d.getwd()
		if cwdErr != nil {
			slog.Debug("[DEBUG-MCP-CLI] getwd failed during session resolution", "error", cwdErr)
		} else if cwd != "" {
			resolved, resolveErr := d.resolveSessionByCwd(d.pipeName(), cwd)
			if resolveErr != nil {
				slog.Debug("[DEBUG-MCP-CLI] resolve-session-by-cwd failed", "cwd", cwd, "error", resolveErr)
			} else {
				session = resolved
			}
		}
	}
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

func (d mcpCLIDeps) resolveMCPStdioViaIPC(sessionName, mcpName string) (ipc.MCPStdioResolvePayload, error) {
	var resp ipc.TmuxResponse
	var sendErr error
	for attempt := 1; attempt <= ipcResolveMaxRetries; attempt++ {
		resp, sendErr = d.sendIPCRequest(d.pipeName(), ipc.TmuxRequest{
			Command: "mcp-resolve-stdio",
			Flags: map[string]any{
				"session": sessionName,
				"mcp":     mcpName,
			},
		})
		if sendErr == nil {
			if attempt > 1 {
				slog.Debug("[DEBUG-MCP-CLI] ipc resolved after retry",
					"session", sessionName,
					"mcp", mcpName,
					"attempts", attempt,
				)
			}
			break
		}
		if !ipc.IsConnectionError(sendErr) {
			return ipc.MCPStdioResolvePayload{}, fmt.Errorf("ipc request failed: %w", sendErr)
		}
		if attempt == ipcResolveMaxRetries {
			return ipc.MCPStdioResolvePayload{}, fmt.Errorf(
				"myT-x IPC is unavailable after %d attempts. Start myT-x GUI first", ipcResolveMaxRetries)
		}
		slog.Debug("[DEBUG-MCP-CLI] ipc connection failed, retrying",
			"attempt", attempt,
			"maxRetries", ipcResolveMaxRetries,
			"error", sendErr,
		)
		time.Sleep(ipcResolveRetryInterval)
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
