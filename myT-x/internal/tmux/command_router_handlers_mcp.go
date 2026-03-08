package tmux

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"myT-x/internal/ipc"
)

func (r *CommandRouter) handleMCPResolveStdio(req ipc.TmuxRequest) ipc.TmuxResponse {
	if r.opts.ResolveMCPStdio == nil {
		return errResp(errors.New("mcp stdio resolver is unavailable"))
	}

	sessionName, mcpName, err := resolveMCPStdioRequestTarget(req)
	if err != nil {
		return errResp(err)
	}

	resolved, err := r.opts.ResolveMCPStdio(sessionName, mcpName)
	if err != nil {
		return errResp(err)
	}
	raw, err := json.Marshal(resolved)
	if err != nil {
		return errResp(fmt.Errorf("encode mcp resolve payload: %w", err))
	}
	return okResp(string(raw))
}

func resolveMCPStdioRequestTarget(req ipc.TmuxRequest) (string, string, error) {
	sessionName, hasSessionFlag, err := optionalMCPResolveFlag(req.Flags, "session")
	if err != nil {
		return "", "", err
	}
	mcpName, hasMCPFlag, err := optionalMCPResolveFlag(req.Flags, "mcp")
	if err != nil {
		return "", "", err
	}
	if hasSessionFlag || hasMCPFlag {
		if !hasSessionFlag || !hasMCPFlag || sessionName == "" || mcpName == "" {
			return "", "", errors.New("session and mcp must both be provided when using flags")
		}
		if len(req.Args) > 0 {
			return "", "", errors.New("session and mcp positional args cannot be mixed with flags")
		}
		return sessionName, mcpName, nil
	}
	if len(req.Args) != 2 {
		return "", "", fmt.Errorf("expected 2 positional arguments, got %d", len(req.Args))
	}
	sessionName = strings.TrimSpace(req.Args[0])
	mcpName = strings.TrimSpace(req.Args[1])
	if sessionName == "" || mcpName == "" {
		return "", "", errors.New("session and mcp are required")
	}
	return sessionName, mcpName, nil
}

func optionalMCPResolveFlag(flags map[string]any, key string) (string, bool, error) {
	if flags == nil {
		return "", false, nil
	}
	value, ok := flags[key]
	if !ok {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", true, fmt.Errorf("%s flag must be a string, got %T", key, value)
	}
	return strings.TrimSpace(text), true, nil
}
