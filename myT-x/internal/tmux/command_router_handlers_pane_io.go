// command_router_handlers_pane_io.go — Pane I/O handlers: send-keys, copy-mode.
package tmux

import (
	"fmt"
	"log/slog"

	"myT-x/internal/ipc"
)

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
	// -M: mouse passthrough. myT-x uses xterm.js for mouse handling;
	// -M is accepted but has no backend effect (log-only no-op).
	// Checked before Terminal nil check because -M doesn't need a terminal.
	if mustBool(req.Flags["-M"]) {
		slog.Debug("[DEBUG-SENDKEYS] -M flag: mouse passthrough is no-op in myT-x",
			"targetPane", target.IDString())
		return okResp("")
	}

	// NOTE: target.Terminal is a live-pointer read after lock release. This is safe in
	// practice because Terminal is set once during pane creation (attachPaneTerminal)
	// and cleared only when the pane is killed. A concurrent kill between ResolveTarget
	// and Write would cause Write to fail gracefully rather than panic.
	if target.Terminal == nil {
		return errResp(fmt.Errorf("pane has no terminal: %s", target.IDString()))
	}

	// -X: copy-mode command dispatch.
	// myT-x uses xterm.js and does not implement full copy-mode. Known commands
	// (e.g. "cancel") are mapped to key sequences; unknown commands are silently
	// ignored per shim spec (never block on transform failure).
	if mustBool(req.Flags["-X"]) {
		return r.handleSendKeysCopyMode(target, req.Args)
	}

	payload := TranslateSendKeys(req.Args)

	slog.Debug("[DEBUG-SENDKEYS] writing to pane",
		"targetPane", target.IDString(),
		"payloadLen", len(payload),
		"payloadPreview", truncateBytes(payload, 200),
	)

	if len(payload) == 0 {
		slog.Debug("[DEBUG-SENDKEYS] empty payload after translation, skipping write",
			"targetPane", target.IDString(),
			"argsCount", len(req.Args),
		)
		return okResp("")
	}
	// Determine send mode based on flags.
	flagN := mustBool(req.Flags["-N"])
	flagW := mustBool(req.Flags["-W"])
	var mode string
	switch {
	case flagN:
		mode = "crlf"
	case flagW:
		mode = "typewriter"
	default:
		mode = "default"
	}
	slog.Debug("[DEBUG-SENDKEYS] mode selection",
		"flagN", flagN,
		"flagW", flagW,
		"mode", mode,
		"payloadHex", fmt.Sprintf("%x", payload),
	)

	switch mode {
	case "crlf":
		// -N: CRLF mode. Transforms trailing \r to \r\n then writes via typewriter
		// mode. Addresses ConPTY on Windows where the input pipe may require CRLF
		// to generate a proper Enter keypress for interactive TUIs (e.g. Copilot CLI).
		if err := writeSendKeysPayloadCRLF(target.Terminal, payload); err != nil {
			return errResp(err)
		}
	case "typewriter":
		// -W: typewriter mode. Writes payload one byte at a time with micro-delays
		// to prevent burst-mode input issues in interactive TUIs.
		if err := writeSendKeysPayloadTypewriter(target.Terminal, payload); err != nil {
			return errResp(err)
		}
	default:
		if err := writeSendKeysPayload(target.Terminal, payload); err != nil {
			return errResp(err)
		}
	}
	return okResp("")
}

// handleSendKeysCopyMode dispatches a copy-mode command (-X flag).
// Only args[0] is used as the command name; additional arguments are ignored.
// An empty args slice is silently ignored and returns success.
// Known commands are translated to key sequences via copyModeCommandTable.
// Unknown commands are logged and silently succeed (shim spec: no error on unknown).
func (r *CommandRouter) handleSendKeysCopyMode(target *TmuxPane, args []string) ipc.TmuxResponse {
	if len(args) == 0 {
		slog.Debug("[DEBUG-SENDKEYS] -X with no command, ignoring")
		return okResp("")
	}
	command := args[0]
	payload, ok := TranslateCopyModeCommand(command)
	if !ok {
		slog.Debug("[DEBUG-SENDKEYS] unknown copy-mode command, ignoring",
			"command", command,
			"targetPane", target.IDString(),
		)
		return okResp("")
	}

	slog.Debug("[DEBUG-SENDKEYS] copy-mode command translated",
		"command", command,
		"targetPane", target.IDString(),
		"payloadLen", len(payload),
	)
	if err := writeSendKeysPayload(target.Terminal, payload); err != nil {
		return errResp(err)
	}
	return okResp("")
}

// handleCopyMode enters or exits copy mode on a target pane.
// myT-x uses xterm.js for scrollback; this handler emits frontend events
// for UI coordination. Unimplemented flags (-u, -e) are accepted silently.
func (r *CommandRouter) handleCopyMode(req ipc.TmuxRequest) ipc.TmuxResponse {
	target, err := r.resolveTargetFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	quit := mustBool(req.Flags["-q"])
	eventName := "tmux:copy-mode-enter"
	if quit {
		eventName = "tmux:copy-mode-exit"
	}

	paneCtx, ctxErr := r.sessions.GetPaneContextSnapshot(target.ID)
	sessionName := ""
	if ctxErr != nil {
		slog.Warn("[WARN-COPYMODE] failed to get pane context snapshot",
			"paneId", target.IDString(), "error", ctxErr)
	} else {
		sessionName = paneCtx.SessionName
	}

	slog.Debug("[DEBUG-COPYMODE] handleCopyMode",
		"targetPane", target.IDString(),
		"quit", quit,
		"event", eventName,
		"ctxErr", ctxErr,
	)

	r.emitter.Emit(eventName, map[string]any{
		"sessionName": sessionName,
		"paneId":      target.IDString(),
	})
	return okResp("")
}
