package tmux

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"myT-x/internal/ipc"
)

// SplitWindowInternal is a typed fast-path for in-process GUI calls.
func (r *CommandRouter) SplitWindowInternal(targetPaneID string, horizontal bool) (string, error) {
	targetPaneID = strings.TrimSpace(targetPaneID)
	if targetPaneID == "" {
		return "", fmt.Errorf("missing required pane id")
	}

	target, err := r.sessions.ResolveTarget(targetPaneID, -1)
	if err != nil {
		return "", err
	}

	direction := SplitVertical
	if horizontal {
		direction = SplitHorizontal
	}
	newPane, err := r.splitWindowResolved(target, direction, "", nil, nil)
	if err != nil {
		return "", err
	}
	return newPane.IDString(), nil
}

func (r *CommandRouter) splitWindowResolved(target *TmuxPane, direction SplitDirection, workDir string, extraEnv map[string]string, args []string) (*TmuxPane, error) {
	if target == nil || target.Window == nil || target.Window.Session == nil {
		return nil, fmt.Errorf("invalid split target")
	}

	targetPaneID := target.ID
	targetCtx, err := r.sessions.GetPaneContextSnapshot(targetPaneID)
	if err != nil {
		return nil, err
	}

	newPane, err := r.sessions.SplitPane(targetPaneID, direction)
	if err != nil {
		return nil, err
	}

	slog.Debug("[DEBUG-SPLIT] splitWindowResolved: request args",
		"originalArgs", fmt.Sprintf("%v", args),
		"originalWorkDir", workDir,
	)

	env := copyEnvMap(targetCtx.Env)
	mergePaneEnvDefaults(env, r.getPaneEnv())
	for k, v := range extraEnv {
		env[k] = v
	}
	addTmuxEnvironment(env, r.opts.PipeName, r.opts.HostPID, targetCtx.SessionID, newPane.ID, r.ShimAvailable())

	if attachErr := r.attachTerminal(newPane, strings.TrimSpace(workDir), env, nil); attachErr != nil {
		if _, _, rollbackErr := r.sessions.KillPane(newPane.IDString()); rollbackErr != nil {
			slog.Warn("[DEBUG-SPLIT] failed to rollback pane after attachTerminal error",
				"paneId", newPane.IDString(), "attachErr", attachErr, "rollbackErr", rollbackErr)
			return nil, fmt.Errorf("%w (rollback failed: %v)", attachErr, rollbackErr)
		}
		return nil, attachErr
	}
	if strings.TrimSpace(targetCtx.Title) != "" {
		if _, titleErr := r.sessions.RenamePane(newPane.IDString(), targetCtx.Title); titleErr != nil {
			slog.Debug("[DEBUG-SPLIT] failed to copy source pane title", "paneId", newPane.IDString(), "error", titleErr)
		}
	}

	if len(args) > 0 {
		// send-keys bootstrap is best-effort: pane creation already succeeded, and
		// tmux-shim contract prefers forwarding over aborting on transform/write failures.
		payload := TranslateSendKeys(append(args, "Enter"))
		if _, err := newPane.Terminal.Write(payload); err != nil {
			slog.Warn("[DEBUG-SPLIT] initial send-keys failed; continuing with created pane",
				"paneId", newPane.IDString(),
				"session", targetCtx.SessionName,
				"error", err,
			)
		}
	}

	layoutSnapshot, layoutErr := r.sessions.paneLayoutSnapshot(newPane.ID)
	if layoutErr != nil {
		if _, _, rollbackErr := r.sessions.KillPane(newPane.IDString()); rollbackErr != nil {
			slog.Warn("[DEBUG-SPLIT] failed to rollback pane after layout snapshot error",
				"paneId", newPane.IDString(), "layoutErr", layoutErr, "rollbackErr", rollbackErr)
			return nil, fmt.Errorf("%w (rollback failed: %v)", layoutErr, rollbackErr)
		}
		return nil, layoutErr
	}

	r.emitter.Emit("tmux:pane-created", map[string]any{
		"sessionName": targetCtx.SessionName,
		"paneId":      newPane.IDString(),
		"env":         env,
		"layout":      layoutSnapshot,
	})
	r.emitter.Emit("tmux:layout-changed", map[string]any{
		"sessionName": targetCtx.SessionName,
		"layoutTree":  layoutSnapshot,
	})

	return newPane, nil
}

func (r *CommandRouter) handleSplitWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-SPLIT] handleSplitWindow called",
		"target", mustString(req.Flags["-t"]),
		"workDir", mustString(req.Flags["-c"]),
		"args", fmt.Sprintf("%v", req.Args),
		"env", fmt.Sprintf("%v", req.Env),
		"callerPane", req.CallerPane,
	)

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}
	direction := SplitVertical
	if mustBool(req.Flags["-h"]) {
		direction = SplitHorizontal
	}
	newPane, err := r.splitWindowResolved(target, direction, mustString(req.Flags["-c"]), req.Env, req.Args)
	if err != nil {
		return errResp(err)
	}

	// -d: keep focus on original pane (don't switch to new pane)
	if mustBool(req.Flags["-d"]) {
		if err := r.sessions.SetActivePane(target.ID); err != nil {
			slog.Debug("[split-window] failed to restore active pane", "error", err)
		}
	}

	// -P with -F: format output using tmux format variables
	if mustBool(req.Flags["-P"]) {
		format := mustString(req.Flags["-F"])
		if format == "" {
			format = "#{pane_id}"
		}
		return okResp(expandFormat(format, newPane) + "\n")
	}

	return okResp(fmt.Sprintf("%s\n", newPane.IDString()))
}

func (r *CommandRouter) handleSendKeys(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-SENDKEYS] handleSendKeys called",
		"target", mustString(req.Flags["-t"]),
		"callerPane", req.CallerPane,
		"argsCount", len(req.Args),
		"args", fmt.Sprintf("%v", req.Args),
	)

	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}
	if target.Terminal == nil {
		return errResp(fmt.Errorf("pane has no terminal: %s", target.IDString()))
	}
	payload := TranslateSendKeys(req.Args)

	slog.Debug("[DEBUG-SENDKEYS] writing to pane",
		"targetPane", target.IDString(),
		"payloadLen", len(payload),
		"payloadPreview", truncateBytes(payload, 200),
	)

	if len(payload) == 0 {
		return okResp("")
	}
	if _, err := target.Terminal.Write(payload); err != nil {
		return errResp(err)
	}
	return okResp("")
}

func (r *CommandRouter) handleSelectPane(req ipc.TmuxRequest) ipc.TmuxResponse {
	var target *TmuxPane
	var err error

	if t := strings.TrimSpace(mustString(req.Flags["-t"])); t != "" {
		target, err = r.resolveTargetFromRequest(req)
		if err != nil {
			return errResp(err)
		}
	} else {
		target, err = r.resolveDirectionalPane(req)
		if err != nil {
			return errResp(err)
		}
	}

	if target == nil {
		return errResp(fmt.Errorf("invalid pane target"))
	}
	targetCtx, targetCtxErr := r.sessions.GetPaneContextSnapshot(target.ID)
	if targetCtxErr != nil {
		return errResp(targetCtxErr)
	}
	if err := r.sessions.SetActivePane(target.ID); err != nil {
		return errResp(err)
	}

	r.emitter.Emit("tmux:pane-focused", map[string]any{
		"sessionName": targetCtx.SessionName,
		"paneId":      target.IDString(),
	})
	return okResp("")
}

func (r *CommandRouter) handleListPanes(req ipc.TmuxRequest) ipc.TmuxResponse {
	target := mustString(req.Flags["-t"])
	allInSession := mustBool(req.Flags["-s"])
	format := mustString(req.Flags["-F"])
	callerPaneID := ParseCallerPane(req.CallerPane)

	panes, err := r.sessions.ListPanesByWindowTarget(target, callerPaneID, allInSession)
	if err != nil {
		return errResp(err)
	}
	sort.Slice(panes, func(i, j int) bool {
		pi, pj := panes[i], panes[j]
		// Defensive: nil pane entries sort to the end.
		if pi == nil || pj == nil {
			return pi != nil
		}
		// Same window pointer (includes both-nil): sort by pane index.
		if pi.Window == pj.Window {
			return pi.Index < pj.Index
		}
		// One window nil: nil-window panes sort to the end.
		if pi.Window == nil || pj.Window == nil {
			return pi.Window != nil
		}
		// Both windows non-nil — compare sessions.
		ls, rs := pi.Window.Session, pj.Window.Session
		if ls == rs {
			// Same session pointer (includes both-nil): sort by window ID.
			return pi.Window.ID < pj.Window.ID
		}
		// One session nil: nil-session panes sort to the end.
		if ls == nil || rs == nil {
			return ls != nil
		}
		return ls.ID < rs.ID
	})

	lines := make([]string, 0, len(panes))
	for _, pane := range panes {
		lines = append(lines, formatPaneLine(pane, format))
	}
	return okResp(joinLines(lines))
}
