package tmux

import (
	"fmt"
	"log/slog"
	"strings"

	"myT-x/internal/ipc"
)

// handleSetOption keeps tmux-compatible scripts working while persisting the
// small compatibility subset of option state required by current workflows.
func (r *CommandRouter) handleSetOption(req ipc.TmuxRequest) ipc.TmuxResponse {
	quiet := mustBool(req.Flags["-q"])
	if len(req.Args) == 0 {
		return compatOptionErrorResp("set-option", quiet, fmt.Errorf("set-option requires an option name"))
	}

	optionName := strings.TrimSpace(req.Args[0])
	if optionName == "" {
		return compatOptionErrorResp("set-option", quiet, fmt.Errorf("set-option requires a non-empty option name"))
	}

	scope, err := r.resolveCompatOptionScope(req)
	if err != nil {
		return compatOptionErrorResp("set-option", quiet, err)
	}

	if mustBool(req.Flags["-u"]) {
		if r.options.unsetOption(scope, optionName) {
			slog.Debug("[DEBUG-OPTION] option reset to compatibility default",
				"option", optionName,
				"scope", scope.kind,
			)
		} else {
			return compatOptionErrorResp("set-option", quiet, fmt.Errorf("unknown option: %s", optionName))
		}
		return okResp("")
	}

	if len(req.Args) < 2 {
		return compatOptionErrorResp("set-option", quiet, fmt.Errorf("set-option requires a value for %s", optionName))
	}

	optionValue := strings.TrimSpace(req.Args[1])
	if optionValue == "" {
		return compatOptionErrorResp("set-option", quiet, fmt.Errorf("set-option requires a non-empty value for %s", optionName))
	}

	if r.options.setOption(scope, optionName, optionValue, mustBool(req.Flags["-o"])) {
		slog.Debug("[DEBUG-OPTION] compatibility option updated",
			"option", optionName,
			"value", optionValue,
			"scope", scope.kind,
		)
		return okResp("")
	}

	return compatOptionErrorResp("set-option", quiet, fmt.Errorf("unsupported option or value: %s=%s", optionName, optionValue))
}

func (r *CommandRouter) handleShowOptions(req ipc.TmuxRequest) ipc.TmuxResponse {
	valueOnly := mustBool(req.Flags["-v"])
	quiet := mustBool(req.Flags["-q"])
	scope, err := r.resolveCompatOptionScope(req)
	if err != nil {
		return compatOptionErrorResp("show-options", quiet, err)
	}

	if len(req.Args) == 0 || strings.TrimSpace(req.Args[0]) == "" {
		lines := make([]string, 0, len(supportedCompatOptionNames()))
		for _, optionName := range supportedCompatOptionNames() {
			value, _ := r.options.getOption(scope, optionName)
			lines = append(lines, formatShowOptionLine(optionName, value, valueOnly))
		}
		return okResp(joinLines(lines))
	}

	optionName := strings.TrimSpace(req.Args[0])
	value, ok := r.options.getOption(scope, optionName)
	if !ok {
		return compatOptionErrorResp("show-options", quiet, fmt.Errorf("unknown option: %s", optionName))
	}

	return okResp(formatShowOptionLine(optionName, value, valueOnly) + "\n")
}

func formatShowOptionLine(optionName string, value string, valueOnly bool) string {
	if valueOnly {
		return value
	}
	return fmt.Sprintf("%s %s", optionName, value)
}

// handleSelectLayout accepts tmux layout commands but treats them as a no-op
// because pane layout selection is managed by the application UI.
func (r *CommandRouter) handleSelectLayout(req ipc.TmuxRequest) ipc.TmuxResponse {
	slog.Debug("[DEBUG-OPTION] select-layout ignored (tmux compatibility no-op)",
		"flags", req.Flags,
		"args", req.Args,
	)
	return okResp("")
}

func compatOptionErrorResp(commandName string, quiet bool, err error) ipc.TmuxResponse {
	if quiet {
		slog.Debug("[DEBUG-OPTION] quiet compatibility option error swallowed",
			"command", commandName,
			"error", err,
		)
		return okResp("")
	}
	return errResp(err)
}

func (r *CommandRouter) resolveCompatOptionScope(req ipc.TmuxRequest) (compatOptionScope, error) {
	scopeFlags := []struct {
		flag string
		kind compatOptionScopeKind
	}{
		{flag: "-g", kind: compatOptionScopeGlobal},
		{flag: "-s", kind: compatOptionScopeSession},
		{flag: "-w", kind: compatOptionScopeWindow},
		{flag: "-p", kind: compatOptionScopePane},
	}

	scope := compatOptionScope{kind: compatOptionScopeGlobal}
	explicitCount := 0
	for _, candidate := range scopeFlags {
		if !mustBool(req.Flags[candidate.flag]) {
			continue
		}
		scope.kind = candidate.kind
		explicitCount++
	}
	if explicitCount > 1 {
		return compatOptionScope{}, fmt.Errorf("set/show-option accepts only one scope flag")
	}

	target := strings.TrimSpace(mustString(req.Flags["-t"]))
	if explicitCount == 0 && target != "" {
		scope.kind = compatOptionScopeSession
	}
	if scope.kind == compatOptionScopeGlobal {
		if target != "" {
			return compatOptionScope{}, fmt.Errorf("global option scope does not accept -t")
		}
		return scope, nil
	}

	switch scope.kind {
	case compatOptionScopeSession:
		session, err := r.resolveCompatOptionSession(req, target)
		if err != nil {
			return compatOptionScope{}, err
		}
		scope.sessionID = session.ID
	case compatOptionScopeWindow:
		pane, err := r.resolveCompatOptionPaneContext(req, target)
		if err != nil {
			return compatOptionScope{}, err
		}
		scope.sessionID = pane.Window.Session.ID
		scope.windowID = pane.Window.ID
	case compatOptionScopePane:
		pane, err := r.resolveCompatOptionPaneContext(req, target)
		if err != nil {
			return compatOptionScope{}, err
		}
		scope.sessionID = pane.Window.Session.ID
		scope.windowID = pane.Window.ID
		scope.paneID = pane.ID
	}
	return scope, nil
}

func (r *CommandRouter) resolveCompatOptionSession(req ipc.TmuxRequest, target string) (*TmuxSession, error) {
	if target != "" {
		return r.sessions.ResolveSessionTarget(target)
	}
	pane, err := r.resolveCompatOptionPaneContext(req, "")
	if err != nil {
		return nil, err
	}
	if pane.Window == nil || pane.Window.Session == nil {
		return nil, fmt.Errorf("failed to resolve session for option scope")
	}
	return pane.Window.Session, nil
}

func (r *CommandRouter) resolveCompatOptionPaneContext(req ipc.TmuxRequest, target string) (*TmuxPane, error) {
	pane, err := r.sessions.ResolveTarget(target, ParseCallerPane(req.CallerPane))
	if err != nil {
		return nil, err
	}
	if pane == nil || pane.Window == nil || pane.Window.Session == nil {
		return nil, fmt.Errorf("failed to resolve pane context for option scope")
	}
	return pane, nil
}
