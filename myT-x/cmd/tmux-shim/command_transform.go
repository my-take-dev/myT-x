package main

import (
	"slices"
	"strings"

	"myT-x/internal/ipc"
	shellparser "myT-x/internal/shell"
)

// applyShellTransform normalizes tmux command arguments for Windows shell
// execution. It mutates req in-place and reports whether a change was applied.
func applyShellTransform(req *ipc.TmuxRequest) bool {
	if req == nil {
		return false
	}
	if req.Flags == nil {
		req.Flags = map[string]any{}
	}
	if req.Env == nil {
		req.Env = map[string]string{}
	}

	switch strings.ToLower(strings.TrimSpace(req.Command)) {
	case "new-session", "new-window", "split-window":
		return applyNewProcessTransform(req)
	case "send-keys":
		return applySendKeysTransform(req)
	default:
		return false
	}
}

func applyNewProcessTransform(req *ipc.TmuxRequest) bool {
	// Defense-in-depth: nil-initialize here even though applyShellTransform
	// (the current sole caller) already does so. This guards against future
	// callers that invoke applyNewProcessTransform directly.
	if req.Flags == nil {
		req.Flags = map[string]any{}
	}
	if req.Env == nil {
		req.Env = map[string]string{}
	}

	workDir := flagValue(req.Flags["-c"])
	parsed := shellparser.ParseUnixCommand(req.Args, workDir)

	changed := false
	if parsed.WorkDir != "" && parsed.WorkDir != workDir {
		req.Flags["-c"] = parsed.WorkDir
		changed = true
	}

	for k, v := range parsed.ExtraEnv {
		current, ok := req.Env[k]
		if !ok || current != v {
			changed = true
		}
		req.Env[k] = v
	}

	if !slices.Equal(req.Args, parsed.CleanArgs) {
		req.Args = append([]string(nil), parsed.CleanArgs...)
		changed = true
	}

	return changed
}

func applySendKeysTransform(req *ipc.TmuxRequest) bool {
	translated := shellparser.TranslateSendKeysArgs(req.Args)
	if slices.Equal(req.Args, translated) {
		return false
	}
	req.Args = translated
	return true
}

func flagValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
