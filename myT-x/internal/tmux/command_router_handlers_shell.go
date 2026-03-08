package tmux

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"myT-x/internal/ipc"
)

// handleRunShell executes a shell command or tmux command and returns its output.
// Flags: -b (background), -t (target for format context), -C (tmux commands), -c (work dir).
// The command string is taken from req.Args.
func (r *CommandRouter) handleRunShell(req ipc.TmuxRequest) ipc.TmuxResponse {
	if len(req.Args) == 0 {
		return errResp(fmt.Errorf("run-shell requires a command argument"))
	}

	command := strings.Join(req.Args, " ")
	workDir := mustString(req.Flags["-c"])
	background := mustBool(req.Flags["-b"])
	tmuxCommands := mustBool(req.Flags["-C"])

	// Expand format variables in the command string if a target pane is available.
	target := mustString(req.Flags["-t"])
	if target != "" {
		pane, resolveErr := r.resolveTargetFromRequest(req)
		if resolveErr == nil {
			command = expandFormatSafe(command, pane.ID, r.sessions)
		}
	}

	slog.Debug("[DEBUG-RUNSHELL] handleRunShell",
		"command", command,
		"workDir", workDir,
		"background", background,
		"tmuxCommands", tmuxCommands,
	)

	// -C: treat argument as tmux commands, not shell command.
	if tmuxCommands {
		return r.runShellAsTmuxCommands(command, background)
	}

	if background {
		go func() {
			stdout, exitCode, err := executeShellCommand(command, workDir)
			if err != nil {
				slog.Debug("[DEBUG-RUNSHELL] background command failed",
					"command", command,
					"exitCode", exitCode,
					"error", err,
				)
			} else {
				slog.Debug("[DEBUG-RUNSHELL] background command completed",
					"command", command,
					"exitCode", exitCode,
					"outputLen", len(stdout),
				)
			}
		}()
		return okResp("")
	}

	stdout, exitCode, err := executeShellCommand(command, workDir)
	if err != nil {
		slog.Debug("[DEBUG-RUNSHELL] command failed",
			"command", command,
			"exitCode", exitCode,
			"error", err,
		)
		return ipc.TmuxResponse{
			ExitCode: exitCode,
			Stderr:   err.Error() + "\n",
		}
	}
	return ipc.TmuxResponse{
		ExitCode: exitCode,
		Stdout:   stdout,
	}
}

// runShellAsTmuxCommands parses a string as tmux command(s) and dispatches them.
// Multiple commands separated by ';' are supported (matching tmux behavior).
// When multiple commands are executed, only the last command's response is returned.
// When background is true, commands are dispatched asynchronously and the caller
// receives an empty success response immediately.
// Semicolons inside quoted strings are preserved as literal characters.
// Flags are parsed using internalCommandFlagSpecs for proper TmuxRequest construction.
func (r *CommandRouter) runShellAsTmuxCommands(commands string, background bool) ipc.TmuxResponse {
	execute := func() ipc.TmuxResponse {
		parts := splitTmuxCommands(commands)
		var lastResp ipc.TmuxResponse
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			tmuxReq := parseTmuxCommandLine(part)
			if tmuxReq.Command == "" {
				continue
			}
			lastResp = r.Execute(tmuxReq)
			if lastResp.ExitCode != 0 {
				slog.Debug("[DEBUG-RUNSHELL] tmux command failed in chain",
					"command", tmuxReq.Command, "part", part,
					"exitCode", lastResp.ExitCode, "stderr", lastResp.Stderr)
			}
		}
		return lastResp
	}

	if background {
		go func() {
			resp := execute()
			if resp.ExitCode != 0 {
				slog.Debug("[DEBUG-RUNSHELL] background tmux command failed",
					"commands", commands,
					"exitCode", resp.ExitCode,
					"stderr", resp.Stderr,
				)
			}
		}()
		return okResp("")
	}
	return execute()
}

// handleIfShell conditionally executes tmux commands based on a shell command exit code
// or a format condition.
// Args: [condition, then-command, else-command?]
// Flags: -F (format condition), -b (background), -t (target for format context).
func (r *CommandRouter) handleIfShell(req ipc.TmuxRequest) ipc.TmuxResponse {
	if len(req.Args) < 2 {
		return errResp(fmt.Errorf("if-shell requires at least 2 arguments: condition and then-command"))
	}

	condition := req.Args[0]
	thenCmd := req.Args[1]
	var elseCmd string
	if len(req.Args) >= 3 {
		elseCmd = req.Args[2]
	}

	background := mustBool(req.Flags["-b"])
	formatCondition := mustBool(req.Flags["-F"])

	slog.Debug("[DEBUG-IFSHELL] handleIfShell",
		"condition", condition,
		"thenCmd", thenCmd,
		"elseCmd", elseCmd,
		"formatCondition", formatCondition,
		"background", background,
	)

	evaluate := func() ipc.TmuxResponse {
		var conditionMet bool

		if formatCondition {
			// -F: evaluate condition as format expression (truthy check).
			// Expand format variables if a target pane is available.
			expanded := condition
			target := mustString(req.Flags["-t"])
			if target != "" {
				pane, resolveErr := r.resolveTargetFromRequest(req)
				if resolveErr == nil {
					expanded = expandFormatSafe(condition, pane.ID, r.sessions)
				}
			}
			conditionMet = expanded != "" && expanded != "0"
		} else {
			// Execute condition as shell command; exit code 0 = true.
			_, exitCode, shellErr := executeShellCommand(condition, "")
			if shellErr != nil {
				slog.Debug("[DEBUG-IFSHELL] condition shell command error",
					"condition", condition, "exitCode", exitCode, "error", shellErr)
			}
			conditionMet = exitCode == 0
		}

		var cmdToRun string
		if conditionMet {
			cmdToRun = thenCmd
		} else {
			cmdToRun = elseCmd
		}

		if cmdToRun == "" {
			return okResp("")
		}

		// Dispatch as tmux command.
		return r.runShellAsTmuxCommands(cmdToRun, false)
	}

	if background {
		go func() {
			resp := evaluate()
			if resp.ExitCode != 0 {
				slog.Debug("[DEBUG-IFSHELL] background if-shell failed",
					"condition", condition,
					"exitCode", resp.ExitCode,
				)
			}
		}()
		return okResp("")
	}
	return evaluate()
}

// executeShellCommand runs a command via the system shell and returns its output.
// On Windows, uses cmd.exe /C. Returns stdout, exit code, and error.
func executeShellCommand(command string, workDir string) (string, int, error) {
	cmd := exec.Command("cmd.exe", "/C", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", 1, err
		}
	}
	return string(output), exitCode, nil
}
