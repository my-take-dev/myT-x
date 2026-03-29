// command_router_handlers_window_lifecycle.go — Window lifecycle handlers: new-window, kill-window, select-window.
package tmux

import (
	"fmt"
	"log/slog"
	"strings"

	"myT-x/internal/ipc"
)

// copySessionFlags copies session-level flags (IsAgentTeam, UseClaudeEnv, UsePaneEnv)
// from the parent session to the newly created child session. Returns the rollback
// stage name and error if any flag copy fails, or empty string and nil on success.
//
// NOTE: When adding new inheritable flags, update this helper to avoid copy-omission bugs.
func copySessionFlags(sessions *SessionManager, parent *TmuxSession, newSessionName string) (string, error) {
	if parent.IsAgentTeam {
		if err := sessions.SetAgentTeam(newSessionName, true); err != nil {
			return "set-agent-team", err
		}
	}
	if parent.UseClaudeEnv != nil {
		if err := sessions.SetUseClaudeEnv(newSessionName, *parent.UseClaudeEnv); err != nil {
			return "set-use-claude-env", err
		}
	}
	if parent.UsePaneEnv != nil {
		if err := sessions.SetUsePaneEnv(newSessionName, *parent.UsePaneEnv); err != nil {
			return "set-use-pane-env", err
		}
	}
	return "", nil
}

// handleNewWindow creates a new child session from a parent session, inheriting
// session-level flags. The 13-step process is:
//
//  1. Resolve parent session from -t flag
//  2. Validate -n as new session name
//  3. Fetch parent session (deep clone)
//  4. Check for duplicate session name
//  5. Read terminal size from parent's active pane
//  6. Create the new session
//  7. Copy session flags from parent (IsAgentTeam, UseClaudeEnv, UsePaneEnv)
//  8. Resolve environment variables for the new pane
//  9. Attach pane terminal
//  10. Send bootstrap keys (best-effort)
//  11. Set active pane unless -d is specified
//  12. Emit tmux:session-created event
//  13. Return formatted output if -P is specified
func (r *CommandRouter) handleNewWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	// 1. -t から親セッションを解決（フラグ継承用、必須）
	parentTarget := strings.TrimSpace(mustString(req.Flags["-t"]))
	if parentTarget == "" {
		return errResp(fmt.Errorf("new-window requires -t with parent session name"))
	}
	parentSessionName := parseSessionName(parentTarget)

	// 2. -n を新セッション名として使用（必須）
	newSessionName := strings.TrimSpace(mustString(req.Flags["-n"]))
	if newSessionName == "" {
		return errResp(fmt.Errorf("new-window requires -n flag"))
	}

	// 3. 親セッション取得（GetSession は deep clone を返す）
	//    HasSession + GetSession の TOCTOU を排除（チェックリスト#69）
	parentSession, ok := r.sessions.GetSession(parentSessionName)
	if !ok {
		return errResp(fmt.Errorf("parent session not found: %s", parentSessionName))
	}

	// 4. セッション名重複チェック
	// NOTE(TOCTOU): この GetSession と後続の CreateSession の間に別ゴルーチンが
	// 同名セッションを作成する理論上のレースがある。CreateSession 内部でロック下に
	// 原子的に重複チェック+作成を行うため、ここでの事前チェックは早期エラーの
	// UX 改善目的であり、一貫性は CreateSession が保証する（チェックリスト#83: 許容パターン）。
	if _, exists := r.sessions.GetSession(newSessionName); exists {
		return errResp(fmt.Errorf("session already exists: %s", newSessionName))
	}

	// 5. 親セッションの active pane からターミナルサイズ取得
	width, height := DefaultTerminalCols, DefaultTerminalRows
	if activePane, activePaneErr := activePaneInSession(parentSession); activePaneErr == nil {
		width = activePane.Width
		height = activePane.Height
	} else {
		// チェックリスト#10: エラーをログのみで処理しデフォルトサイズで続行
		slog.Debug("[DEBUG-WINDOW] using default terminal size, parent active pane unavailable",
			"parent", parentSessionName, "error", activePaneErr)
	}

	// 6. 新セッション作成
	workDir := mustString(req.Flags["-c"])
	session, pane, err := r.sessions.CreateSession(newSessionName, "0", width, height)
	if err != nil {
		return errResp(err)
	}

	// ロールバック関数（handleNewSession と同パターン）
	// rollbackSession removes the child session created in step 6 when any later
	// step fails (flag copy, snapshot refresh, terminal attach). This prevents
	// partially initialized sessions from being left in SessionManager.
	//
	// NOTE: The stage parameter is included only in slog.Warn when rollback itself
	// fails (RemoveSession error). On successful rollback, the ipc.TmuxResponse
	// contains only originalErr -- stage is intentionally omitted from the client-
	// facing error because it is an internal diagnostic detail. This matches
	// handleNewSession's rollbackSession behavior.
	rollbackSession := func(stage string, originalErr error) ipc.TmuxResponse {
		if _, rmErr := r.sessions.RemoveSession(session.Name); rmErr != nil {
			slog.Warn("[WINDOW] failed to remove session during rollback",
				"session", session.Name, "stage", stage, "originalErr", originalErr, "removeErr", rmErr)
			return errResp(originalErr)
		}
		return errResp(originalErr)
	}

	// 7. 親セッションからフラグ継承（copySessionFlags に集約）
	if stage, setErr := copySessionFlags(r.sessions, parentSession, newSessionName); setErr != nil {
		return rollbackSession(stage, setErr)
	}

	// 8. 環境変数解決（フラグ継承後の新セッションスナップショットで解決）
	paneCtx, paneCtxErr := r.sessions.GetPaneContextSnapshot(pane.ID)
	if paneCtxErr != nil {
		slog.Debug("[DEBUG-WINDOW] failed to get pane context after CreateSession", "error", paneCtxErr)
		return rollbackSession("snapshot", paneCtxErr)
	}

	// フラグ継承後のセッションを明示的に再取得して env 解決に渡す
	// (split-window と同パターン: nil 回避で暗黙的な内部 GetSession を排除)
	getSession := r.getSessionForNewWindowFn
	// Defensive fallback: NewCommandRouter wires this seam by default, but tests
	// and future constructors may leave it nil.
	if getSession == nil {
		getSession = r.sessions.GetSession
	}
	newSessionSnap, snapOk := getSession(newSessionName)
	if !snapOk {
		return rollbackSession("snapshot-refetch", fmt.Errorf("session disappeared during setup: %s", newSessionName))
	}
	// NOTE(1-window model): New sessions start with a fresh environment.
	// inheritedEnv is nil because there is no parent pane to inherit from
	// in the 1-session-per-window model.
	env := r.resolveEnvForPaneCreation(newSessionSnap, newSessionName, nil, req.Env, paneCtx.SessionID, pane.ID)

	// 9. ターミナル接続
	if attachErr := r.attachPaneTerminal(pane, workDir, env, nil); attachErr != nil {
		return rollbackSession("attach-terminal", attachErr)
	}

	// 10. send-keys bootstrap（best-effort）
	r.bestEffortSendKeys(pane, req.Args, true, "DEBUG-WINDOW", newSessionName)

	// 11. -d でなければアクティブペイン設定 + フォーカスイベント発行
	//     handleSelectWindow / handleSelectPane と同一パターン:
	//     SetActivePane 成功時に tmux:pane-focused を emit する。
	if !mustBool(req.Flags["-d"]) {
		if setErr := r.sessions.SetActivePane(pane.ID); setErr != nil {
			slog.Warn("[WINDOW] SetActivePane failed after new-window",
				"paneId", pane.IDString(), "error", setErr)
		} else {
			r.emitter.Emit("tmux:pane-focused", map[string]any{
				"sessionName": newSessionName,
				"paneId":      pane.IDString(),
			})
		}
	}

	// 12. セッション作成イベント emit
	emitCtx, emitCtxErr := r.sessions.GetPaneContextSnapshot(pane.ID)
	if emitCtxErr != nil {
		slog.Debug("[DEBUG-WINDOW] failed to refresh pane context for session-created event",
			"session", newSessionName, "error", emitCtxErr)
		emitCtx = paneCtx
	}
	r.emitter.Emit("tmux:session-created", map[string]any{
		"name":          emitCtx.SessionName,
		"id":            emitCtx.SessionID,
		"initialPane":   pane.IDString(),
		"initialLayout": emitCtx.Layout,
	})

	// 13. -P/-F: フォーマット出力
	if mustBool(req.Flags["-P"]) {
		format := mustString(req.Flags["-F"])
		if format == "" {
			format = "#{session_name}:#{window_index}"
		}
		return okResp(expandFormatSafe(format, pane.ID, r.sessions) + "\n")
	}

	return okResp("")
}

func (r *CommandRouter) handleKillWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	sessionName, windowID, err := r.resolveWindowIDFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	// I-15: survivingWindowID is computed atomically inside RemoveWindowByID
	// under the same lock as the removal, eliminating the TOCTOU race of
	// pre-computing fallback from a clone then removing from live data.
	result, removeErr := r.sessions.RemoveWindowByID(sessionName, windowID)
	if removeErr != nil {
		return errResp(removeErr)
	}

	// Close terminals outside the session manager lock.
	for _, pane := range result.RemovedPanes {
		if pane == nil || pane.Terminal == nil {
			continue
		}
		if closeErr := pane.Terminal.Close(); closeErr != nil {
			slog.Warn("[WINDOW] terminal close failed during kill-window",
				"paneId", pane.IDString(), "error", closeErr)
		}
	}

	if result.SessionRemoved {
		r.emitter.Emit("tmux:session-destroyed", map[string]any{
			"name": sessionName,
		})
	} else if result.SessionEmptied {
		r.emitter.Emit("tmux:session-emptied", map[string]any{
			"name": sessionName,
		})
	} else {
		// NOTE(1-window model): このブランチは主にテスト経由で到達する。
		// 通常実行では最後のウィンドウ削除は空セッション遷移となり、
		// マルチウィンドウ時のみ window-destroyed が発火する。
		r.emitter.Emit("tmux:window-destroyed", map[string]any{
			"sessionName": sessionName,
			"windowId":    windowID,
		})
		r.emitLayoutChangedForSession(sessionName, result.SurvivingWindowID, "DEBUG-KILLWINDOW")
	}

	return okResp("")
}

func (r *CommandRouter) handleSelectWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
	targetText := strings.TrimSpace(mustString(req.Flags["-t"]))
	if targetText == "" {
		return errResp(fmt.Errorf("missing required flag: -t"))
	}

	sessionName, windowID, err := r.resolveWindowIDFromRequest(req)
	if err != nil {
		return errResp(err)
	}

	// GetSession returns a deep clone. Pane IDs from the clone are stable scalars
	// safe to use in SetActivePane even if the session is concurrently modified.
	session, ok := r.sessions.GetSession(sessionName)
	if !ok {
		return errResp(fmt.Errorf("session not found: %s", sessionName))
	}

	window, _ := findWindowByID(session.Windows, windowID)
	if window == nil {
		return errResp(fmt.Errorf("window not found in session: %s", sessionName))
	}

	pane, paneErr := activePaneInWindow(window)
	if paneErr != nil {
		return errResp(paneErr)
	}

	if setErr := r.sessions.SetActivePane(pane.ID); setErr != nil {
		return errResp(setErr)
	}

	// Focus events are intentionally pane-scoped: SetActivePane updates both active pane and
	// ActiveWindowID, and consumers should derive window focus changes from snapshot deltas.
	r.emitter.Emit("tmux:pane-focused", map[string]any{
		"sessionName": sessionName,
		"paneId":      pane.IDString(),
	})
	return okResp("")
}
