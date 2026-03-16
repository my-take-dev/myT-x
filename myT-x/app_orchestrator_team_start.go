package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"myT-x/internal/tmux"
)

const (
	orchestratorLaunchModeActiveSession LaunchMode = "active_session"
	orchestratorLaunchModeNewSession    LaunchMode = "new_session"
	orchestratorTeamShellInitDelay                 = 500 * time.Millisecond
	orchestratorTeamCdDelay                        = 300 * time.Millisecond
	orchestratorTeamBootstrapDelay                 = 3 * time.Second
)

var (
	orchestratorAgentNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)
	orchestratorTeamSleepFn        = time.Sleep

	createSessionForOrchestratorTeamFn = func(a *App, rootPath, sessionName string, opts CreateSessionOptions) (tmux.SessionSnapshot, error) {
		return a.CreateSession(rootPath, sessionName, opts)
	}
	killSessionForOrchestratorTeamFn = func(a *App, sessionName string, deleteWorktree bool) error {
		return a.KillSession(sessionName, deleteWorktree)
	}
	splitPaneForOrchestratorTeamFn = func(a *App, paneID string, horizontal bool) (string, error) {
		return a.SplitPane(paneID, horizontal)
	}
	renamePaneForOrchestratorTeamFn = func(a *App, paneID, title string) error {
		return a.RenamePane(paneID, title)
	}
	sendKeysForOrchestratorTeamFn = func(router *tmux.CommandRouter, paneID string, text string) error {
		return sendKeysLiteralWithEnter(router, paneID, text)
	}
	sendKeysPasteForOrchestratorTeamFn = func(router *tmux.CommandRouter, paneID string, text string) error {
		return sendKeysLiteralPasteWithEnter(router, paneID, text)
	}
	applyLayoutPresetForOrchestratorTeamFn = func(a *App, sessionName, preset string) error {
		return a.ApplyLayoutPreset(sessionName, preset)
	}
)

// StartOrchestratorTeam は指定チームのエージェントを起動する。
func (a *App) StartOrchestratorTeam(request StartOrchestratorTeamRequest) (StartOrchestratorTeamResult, error) {
	request.Normalize()
	if err := request.Validate(); err != nil {
		return StartOrchestratorTeamResult{}, err
	}

	sourceSessionName := request.SourceSessionName
	if sourceSessionName == "" {
		sourceSessionName = a.getActiveSessionName()
	}

	teams, err := a.LoadOrchestratorTeams(sourceSessionName)
	if err != nil {
		return StartOrchestratorTeamResult{}, err
	}
	team, err := findOrchestratorTeamByID(teams, request.TeamID)
	if err != nil {
		return StartOrchestratorTeamResult{}, err
	}
	if len(team.Members) == 0 {
		return StartOrchestratorTeamResult{}, errors.New("team has no members")
	}

	sourceSession, err := a.findSessionSnapshotByName(sourceSessionName)
	if err != nil {
		return StartOrchestratorTeamResult{}, err
	}
	sourceRootPath, err := resolveOrchestratorSourceRootPath(sourceSession)
	if err != nil {
		return StartOrchestratorTeamResult{}, err
	}

	sessionName, panes, createdNewSession, warnings, err := a.prepareOrchestratorTeamLaunchTarget(team, request, sourceRootPath, sourceSession)
	if err != nil {
		if createdNewSession && strings.TrimSpace(sessionName) != "" {
			if rollbackErr := killSessionForOrchestratorTeamFn(a, sessionName, false); rollbackErr != nil {
				slog.Warn("[WARN-ORCH-TEAM] failed to rollback new session after target preparation failure",
					"session", sessionName, "error", rollbackErr)
			}
		}
		return StartOrchestratorTeamResult{}, err
	}

	result := StartOrchestratorTeamResult{
		SessionName:   sessionName,
		LaunchMode:    request.LaunchMode,
		MemberPaneIDs: make(map[string]string, len(team.Members)),
		Warnings:      append([]string{}, warnings...),
	}

	injectedAnyCommand := false
	if createdNewSession {
		defer func() {
			if injectedAnyCommand {
				return
			}
			if err := killSessionForOrchestratorTeamFn(a, sessionName, false); err != nil {
				slog.Warn("[WARN-ORCH-TEAM] failed to rollback new session after pre-launch failure",
					"session", sessionName, "error", err)
			}
		}()
	}

	router := a.router
	if router == nil {
		return StartOrchestratorTeamResult{}, errors.New("command router is not initialized")
	}

	// Wait for shells in newly created panes to initialize.
	orchestratorTeamSleepFn(orchestratorTeamShellInitDelay)

	agentNames := deriveOrchestratorAgentNames(team.Members)
	for index, member := range team.Members {
		if index >= len(panes) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped member %s: no pane available", member.PaneTitle))
			continue
		}

		paneID := panes[index].ID
		result.MemberPaneIDs[member.ID] = paneID

		if err := renamePaneForOrchestratorTeamFn(a, paneID, member.PaneTitle); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to rename pane %s for member %s: %v", paneID, member.PaneTitle, err))
		}

		if strings.TrimSpace(sourceRootPath) != "" {
			// Use double quotes for Windows PowerShell compatibility.
			cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(sourceRootPath, `"`, `\"`))
			slog.Info("[DEBUG-SENDKEYS] cd command", "paneID", paneID, "member", member.PaneTitle, "fullText", cdCommand)
			if err := sendKeysForOrchestratorTeamFn(router, paneID, cdCommand); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("cd failed for member %s in pane %s (skipping launch): %v", member.PaneTitle, paneID, err))
				continue
			}
			orchestratorTeamSleepFn(orchestratorTeamCdDelay)
		}

		launchCommand := buildOrchestratorLaunchCommand(member.Command, member.Args)
		slog.Info("[DEBUG-SENDKEYS] launch command", "paneID", paneID, "member", member.PaneTitle, "fullText", launchCommand)
		if err := sendKeysForOrchestratorTeamFn(router, paneID, launchCommand); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to launch member %s in pane %s: %v", member.PaneTitle, paneID, err))
			continue
		}
		injectedAnyCommand = true
		orchestratorTeamSleepFn(time.Duration(team.BootstrapDelayMs) * time.Millisecond)

		bootstrapMessage := buildOrchestratorBootstrapMessage(team.Name, member, paneID, agentNames[member.ID])
		slog.Info("[DEBUG-SENDKEYS] bootstrap message", "paneID", paneID, "member", member.PaneTitle, "fullText", bootstrapMessage)
		// Claude Code treats \n as Enter/submit in its terminal UI.
		// Use bracketed paste mode so the entire message is received as one input.
		if isClaudeCommand(member.Command) {
			if err := sendKeysPasteForOrchestratorTeamFn(router, paneID, bootstrapMessage); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", member.PaneTitle, paneID, err))
			}
		} else {
			if err := sendKeysForOrchestratorTeamFn(router, paneID, bootstrapMessage); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", member.PaneTitle, paneID, err))
			}
		}
	}

	if len(result.MemberPaneIDs) == 0 {
		return result, errors.New("failed to launch any team member")
	}

	if len(result.Warnings) > 0 {
		slog.Warn("[WARN-ORCH-TEAM] launch completed with warnings",
			"team", team.Name,
			"session", result.SessionName,
			"warningCount", len(result.Warnings))
	} else {
		slog.Info("[DEBUG-ORCH-TEAM] launch completed",
			"team", team.Name,
			"session", result.SessionName,
			"memberCount", len(result.MemberPaneIDs))
	}
	return result, nil
}

func (a *App) prepareOrchestratorTeamLaunchTarget(
	team OrchestratorTeamDefinition,
	request StartOrchestratorTeamRequest,
	sourceRootPath string,
	sourceSession tmux.SessionSnapshot,
) (string, []tmux.PaneSnapshot, bool, []string, error) {
	switch request.LaunchMode {
	case orchestratorLaunchModeActiveSession:
		activeWindow := resolveActiveWindowSnapshot(sourceSession.Windows, sourceSession.ActiveWindowID)
		if activeWindow == nil {
			return "", nil, false, nil, fmt.Errorf("session %s has no active window", sourceSession.Name)
		}
		panes := cloneAndSortOrchestratorPanes(activeWindow.Panes)
		panes, warnings, err := a.ensureOrchestratorTeamPaneCapacity(sourceSession.Name, panes, len(team.Members), true)
		if err != nil {
			return "", nil, false, nil, err
		}
		return sourceSession.Name, panes, false, warnings, nil
	case orchestratorLaunchModeNewSession:
		sessionName := request.NewSessionName
		if sessionName == "" {
			sessionName = sanitizeSessionName(team.Name, "orchestrator-team")
		}
		createdSession, err := createSessionForOrchestratorTeamFn(a, sourceRootPath, sessionName, CreateSessionOptions{})
		if err != nil {
			return "", nil, false, nil, err
		}
		activeWindow := resolveActiveWindowSnapshot(createdSession.Windows, createdSession.ActiveWindowID)
		if activeWindow == nil {
			// Return createdSession.Name so the caller can rollback the session.
			return createdSession.Name, nil, true, nil, fmt.Errorf("session %s has no active window", createdSession.Name)
		}
		panes := cloneAndSortOrchestratorPanes(activeWindow.Panes)
		panes, warnings, err := a.ensureOrchestratorTeamPaneCapacity(createdSession.Name, panes, len(team.Members), false)
		if err != nil {
			return createdSession.Name, nil, true, nil, err
		}
		if len(panes) > 1 {
			if err := applyLayoutPresetForOrchestratorTeamFn(a, createdSession.Name, "tiled"); err != nil {
				return createdSession.Name, nil, true, nil, fmt.Errorf("apply tiled layout: %w", err)
			}
		}
		return createdSession.Name, panes, true, warnings, nil
	default:
		return "", nil, false, nil, fmt.Errorf("unsupported launch mode: %s", request.LaunchMode)
	}
}

func (a *App) ensureOrchestratorTeamPaneCapacity(
	sessionName string,
	panes []tmux.PaneSnapshot,
	requiredCount int,
	allowPartial bool,
) ([]tmux.PaneSnapshot, []string, error) {
	if requiredCount < 1 {
		return []tmux.PaneSnapshot{}, nil, nil
	}
	if len(panes) == 0 {
		return nil, nil, fmt.Errorf("session %s has no panes in the active window", sessionName)
	}

	working := append([]tmux.PaneSnapshot{}, panes...)
	warnings := make([]string, 0)
	for len(working) < requiredCount {
		sourcePaneID := working[len(working)-1].ID
		newPaneID, err := splitPaneForOrchestratorTeamFn(a, sourcePaneID, false)
		if err != nil {
			if !allowPartial {
				return nil, nil, fmt.Errorf("create pane %d for session %s: %w", len(working)+1, sessionName, err)
			}
			remaining := requiredCount - len(working)
			warnings = append(warnings, fmt.Sprintf("failed to create %d additional pane(s) in session %s: %v", remaining, sessionName, err))
			break
		}
		working = append(working, tmux.PaneSnapshot{
			ID:    strings.TrimSpace(newPaneID),
			Index: len(working),
		})
	}
	return working, warnings, nil
}

func (a *App) findSessionSnapshotByName(sessionName string) (tmux.SessionSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return tmux.SessionSnapshot{}, errors.New("source session is required")
	}
	sessions, err := a.requireSessions()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}
	for _, snapshot := range sessions.Snapshot() {
		if snapshot.Name == sessionName {
			return snapshot, nil
		}
	}
	return tmux.SessionSnapshot{}, fmt.Errorf("session %s not found", sessionName)
}

func resolveOrchestratorSourceRootPath(session tmux.SessionSnapshot) (string, error) {
	if session.Name == "" {
		return "", errors.New("session name is empty")
	}
	if session.Worktree != nil {
		if worktreePath := strings.TrimSpace(session.Worktree.Path); worktreePath != "" {
			return worktreePath, nil
		}
	}
	if rootPath := strings.TrimSpace(session.RootPath); rootPath != "" {
		return rootPath, nil
	}
	return "", fmt.Errorf("session %s has no root path or worktree", session.Name)
}

// isClaudeCommand returns true if the command refers to Claude Code CLI.
// Claude Code requires bracketed paste mode for multi-line input
// because it treats \n as Enter/submit in its terminal UI.
func isClaudeCommand(command string) bool {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(parts[0]))
	return base == "claude" || base == "claude.exe" || strings.HasPrefix(base, "claude-code")
}

func buildOrchestratorLaunchCommand(command string, args []string) string {
	parts := []string{strings.TrimSpace(command)}
	for _, arg := range args {
		parts = append(parts, quoteOrchestratorCommandArg(arg))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func quoteOrchestratorCommandArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	// Escape backslashes that precede quotes, then escape quotes.
	var buf strings.Builder
	buf.Grow(len(arg) + 2)
	buf.WriteByte('"')
	for i := 0; i < len(arg); i++ {
		if arg[i] == '\\' {
			// Count consecutive backslashes
			j := i
			for j < len(arg) && arg[j] == '\\' {
				j++
			}
			// Double backslashes if followed by quote or end of string
			if j == len(arg) || arg[j] == '"' {
				for range j - i {
					buf.WriteString(`\\`)
				}
			} else {
				buf.WriteString(arg[i:j])
			}
			i = j - 1
		} else if arg[i] == '"' {
			buf.WriteString(`\"`)
		} else {
			buf.WriteByte(arg[i])
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func buildOrchestratorBootstrapMessage(teamName string, member OrchestratorTeamMember, paneID, agentName string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "あなたは「%s」チームのメンバーです。\n", strings.TrimSpace(teamName))
	fmt.Fprintf(&builder, "役割名: %s\n", member.Role)
	if member.CustomMessage != "" {
		builder.WriteString("\n")
		builder.WriteString(member.CustomMessage)
	}
	if len(member.Skills) > 0 {
		builder.WriteString("\n得意分野:\n")
		for _, skill := range member.Skills {
			if skill.Description != "" {
				fmt.Fprintf(&builder, "- %s: %s\n", skill.Name, skill.Description)
			} else {
				fmt.Fprintf(&builder, "- %s\n", skill.Name)
			}
		}
	}
	builder.WriteString("\n--- エージェント登録 ---\n")
	builder.WriteString("自身のペインIDは環境変数 $TMUX_PANE で確認できます。\n")
	fmt.Fprintf(&builder, "現在のペインID: %s\n", paneID)
	fmt.Fprintf(&builder, "まず以下を実行して自身をオーケストレーターに登録してください:\n")
	if len(member.Skills) > 0 {
		skillsJSON, err := json.Marshal(member.Skills)
		if err != nil {
			slog.Warn("[WARN-ORCH-TEAM] failed to marshal skills for bootstrap", "member", member.PaneTitle, "error", err)
			skillsJSON = []byte("[]")
		}
		fmt.Fprintf(&builder, "register_agent(name=\"%s\", pane_id=\"%s\", role=\"%s\", skills=%s)\n", agentName, paneID, member.Role, string(skillsJSON))
	} else {
		fmt.Fprintf(&builder, "register_agent(name=\"%s\", pane_id=\"%s\", role=\"%s\")\n", agentName, paneID, member.Role)
	}

	// スキル自動補完指示
	if hints := buildSkillCompletionHints(member.Role, member.Skills); hints != "" {
		builder.WriteString("\n--- 得意分野の補完 ---\n")
		builder.WriteString(hints)
	}

	// ワークフローガイド
	builder.WriteString("\n--- ワークフロー ---\n")
	builder.WriteString("1. register_agent → 自身を登録（必須・最初に実行）\n")
	builder.WriteString("2. list_agents → チームメンバーとペイン状態を確認\n")
	builder.WriteString("3. send_task → 他エージェントにタスクを依頼（from_agent=自分の名前）\n")
	builder.WriteString("4. get_my_tasks → 自分宛タスクを確認（デフォルト: pending のみ）\n")
	builder.WriteString("5. send_response → タスクに返信し completed に更新（task_id 必須）\n")
	builder.WriteString("\nタスク状態: pending → completed / failed / abandoned\n")
	builder.WriteString("確認: check_tasks で全タスク一覧、capture_pane で相手の画面を取得\n")
	builder.WriteString("注意: send_task は応答テンプレートを自動付与（include_response_instructions=false で無効化可）\n")

	return builder.String()
}

// buildSkillCompletionHints はスキル状態に応じた自動補完指示を生成する。
func buildSkillCompletionHints(role string, skills []OrchestratorTeamMemberSkill) string {
	var hints []string

	if len(skills) == 0 {
		hints = append(hints, fmt.Sprintf(
			"得意分野（skills）が未設定です。あなたの役割「%s」に基づき、register_agent 実行時に適切な得意分野を3〜5件、name と description 付きで追加してください。",
			role,
		))
	} else {
		// Description が空のスキルがあるか
		hasEmptyDesc := false
		for _, s := range skills {
			if s.Description == "" {
				hasEmptyDesc = true
				break
			}
		}
		if hasEmptyDesc {
			hints = append(hints, fmt.Sprintf(
				"得意分野の一部に説明（description）がありません。あなたの役割「%s」に基づき、register_agent 実行時に不足している description を推測して補完してください。",
				role,
			))
		}

		// スキルが少ない（1〜2件）
		if len(skills) < 3 {
			hints = append(hints, fmt.Sprintf(
				"得意分野が少ない可能性があります。あなたの役割「%s」に応じて、register_agent 実行時に関連する得意分野を追加してください（合計3〜5件を目安）。",
				role,
			))
		}
	}

	if len(hints) == 0 {
		return ""
	}
	return strings.Join(hints, "\n")
}

func deriveOrchestratorAgentNames(members []OrchestratorTeamMember) map[string]string {
	result := make(map[string]string, len(members))
	used := make(map[string]int, len(members))
	for _, member := range members {
		base := sanitizeOrchestratorAgentName(member.PaneTitle)
		used[base]++
		name := base
		if used[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, used[base])
		}
		result[member.ID] = name
	}
	return result
}

func sanitizeOrchestratorAgentName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = orchestratorAgentNameSanitizer.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "member"
	}
	return normalized
}

func cloneAndSortOrchestratorPanes(panes []tmux.PaneSnapshot) []tmux.PaneSnapshot {
	cloned := append([]tmux.PaneSnapshot{}, panes...)
	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].Index != cloned[j].Index {
			return cloned[i].Index < cloned[j].Index
		}
		return cloned[i].ID < cloned[j].ID
	})
	return cloned
}

func findOrchestratorTeamByID(teams []OrchestratorTeamDefinition, teamID string) (OrchestratorTeamDefinition, error) {
	for _, team := range teams {
		if team.ID == teamID {
			return team, nil
		}
	}
	return OrchestratorTeamDefinition{}, fmt.Errorf("team %s not found", teamID)
}
