package ipc

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/user"
	"regexp"
	"strings"

	"myT-x/internal/userutil"
)

var pipeNamePattern = regexp.MustCompile(`(?i)^\\\\\.\\pipe\\myT-x-[a-z0-9._-]{1,128}$`)

const defaultPipePrefix = `\\.\pipe\myT-x-`

// TmuxRequest is a single tmux-compatible command request.
type TmuxRequest struct {
	Command    string            `json:"command"`
	Flags      map[string]any    `json:"flags,omitempty"` // Values are string, bool, or float64 (JSON number); map[string]any for tmux CLI compat
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	CallerPane string            `json:"caller_pane,omitempty"`
}

// TmuxResponse is a tmux-compatible command response.
type TmuxResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

// CommandExecutor handles a tmux request and returns a response.
type CommandExecutor interface {
	Execute(req TmuxRequest) TmuxResponse
}

func sanitizeUsername(value string) string {
	return userutil.SanitizeUsername(value)
}

// DefaultPipeName returns the pipe path to use. If the GO_TMUX_PIPE
// environment variable is set and passes pattern validation, its value is
// used; otherwise a per-user default is constructed from the current username.
func DefaultPipeName() string {
	if v, ok := trustedPipeNameFromEnv(); ok {
		return v
	}

	username := strings.TrimSpace(os.Getenv("USERNAME"))
	if username == "" {
		if current, err := user.Current(); err == nil {
			username = current.Username
		} else {
			// Both USERNAME env and user.Current() unavailable.
			// sanitizeUsername("") returns "unknown" as a safe fallback
			// to avoid generating a bare pipe name like "\\.\pipe\myT-x-".
			// NOTE: "unknown" fallback is extremely unlikely (requires both
			// $USERNAME empty and user.Current() failure). Collision risk
			// between multiple users falling back to the same pipe name
			// is accepted for robustness over failing to start.
			slog.Warn("[ipc] could not determine username, falling back to default",
				"error", err)
		}
	}
	return defaultPipePrefix + sanitizeUsername(username)
}

func trustedPipeNameFromEnv() (string, bool) {
	value := strings.TrimSpace(os.Getenv("GO_TMUX_PIPE"))
	if value == "" {
		return "", false
	}
	if !pipeNamePattern.MatchString(value) {
		slog.Warn("[ipc] GO_TMUX_PIPE rejected: value does not match allowed pattern", "value", value)
		return "", false
	}
	return value, true
}

func encodeRequest(req TmuxRequest) ([]byte, error) {
	return json.Marshal(req)
}

func decodeRequest(raw []byte) (TmuxRequest, error) {
	var req TmuxRequest
	err := json.Unmarshal(raw, &req)
	if err != nil {
		return TmuxRequest{}, err
	}

	// Normalize nil collection fields to empty values so callers never
	// need nil checks. Every collection field in TmuxRequest is initialized:
	//   Flags -> empty map, Args -> empty slice, Env -> empty map.
	if req.Flags == nil {
		req.Flags = map[string]any{}
	}
	if req.Args == nil {
		req.Args = []string{}
	}
	if req.Env == nil {
		req.Env = map[string]string{}
	}
	return req, nil
}

func encodeResponse(resp TmuxResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func decodeResponse(raw []byte) (TmuxResponse, error) {
	var resp TmuxResponse
	err := json.Unmarshal(raw, &resp)
	if err != nil {
		return TmuxResponse{}, err
	}
	return resp, nil
}
