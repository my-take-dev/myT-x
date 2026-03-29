package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

const (
	// BootstrapDelayDefault はブートストラップ遅延のデフォルト値（ms）。
	BootstrapDelayDefault = 3000
	// BootstrapDelayMin はブートストラップ遅延の最小値（ms）。
	BootstrapDelayMin = 1000
	// BootstrapDelayMax はブートストラップ遅延の最大値（ms）。
	BootstrapDelayMax = 30000

	shellInitDelay = 500 * time.Millisecond
	cdDelay        = 300 * time.Millisecond

	defaultTeamName = "動的チーム"
)

// AddMemberCmd は add_member ツールのコマンドパラメータ。
type AddMemberCmd struct {
	PaneTitle        string
	Role             string
	Command          string
	Args             []string
	CustomMessage    string
	Skills           []domain.Skill
	TeamName         string // デフォルト "動的チーム"
	SplitFrom        string // 分割元ペインID（デフォルト: 呼び出し元）
	SplitDirection   string // "horizontal" / "vertical"（デフォルト: "horizontal"）
	BootstrapDelayMs int    // 1000-30000（デフォルト: 3000）
}

// AddMemberResult は add_member ツールの実行結果。
type AddMemberResult struct {
	PaneID    string
	PaneTitle string
	AgentName string
	Warnings  []string
}

// MemberBootstrapService は MCP 経由でメンバーを動的に追加するサービス。
type MemberBootstrapService struct {
	agents      domain.AgentRepository
	resolver    domain.SelfPaneResolver
	splitter    domain.PaneSplitter
	titleSetter domain.PaneTitleSetter
	sender      domain.PaneSender
	pasteSender domain.PanePasteSender
	projectRoot string
	sleepFn     func(context.Context, time.Duration) error
	logger      *log.Logger
}

// NewMemberBootstrapService は MemberBootstrapService を構築する。
func NewMemberBootstrapService(
	agents domain.AgentRepository,
	resolver domain.SelfPaneResolver,
	splitter domain.PaneSplitter,
	titleSetter domain.PaneTitleSetter,
	sender domain.PaneSender,
	pasteSender domain.PanePasteSender,
	projectRoot string,
	logger *log.Logger,
) *MemberBootstrapService {
	return &MemberBootstrapService{
		agents:      agents,
		resolver:    resolver,
		splitter:    splitter,
		titleSetter: titleSetter,
		sender:      sender,
		pasteSender: pasteSender,
		projectRoot: projectRoot,
		sleepFn:     sleepContext,
		logger:      ensureLogger(logger),
	}
}

// SetSleepFn はテスト用に sleep 関数を差し替える。
func (s *MemberBootstrapService) SetSleepFn(fn func(context.Context, time.Duration) error) {
	s.sleepFn = fn
}

// AddMember は新メンバーをペイン分割・CLI起動・ブートストラップメッセージ送信で追加する。
func (s *MemberBootstrapService) AddMember(ctx context.Context, cmd AddMemberCmd) (AddMemberResult, error) {
	// 呼び出し元の解決と登録済みエージェント確認
	caller, err := resolveCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return AddMemberResult{}, err
	}

	// trusted caller（pipe bridge）で split_from 未指定はエラー
	splitFrom := cmd.SplitFrom
	if splitFrom == "" {
		if IsTrustedCaller(caller) {
			return AddMemberResult{}, fmt.Errorf("split_from is required when caller pane is unavailable")
		}
		splitFrom = caller.PaneID
	}

	// デフォルト値
	teamName := cmd.TeamName
	if teamName == "" {
		teamName = defaultTeamName
	}
	horizontal := cmd.SplitDirection != "vertical"
	delayMs := clampBootstrapDelay(cmd.BootstrapDelayMs)

	var warnings []string

	// 1. ペイン分割
	newPaneID, err := s.splitter.SplitPane(ctx, splitFrom, horizontal)
	if err != nil {
		return AddMemberResult{}, operationError(s.logger, "failed to split pane", err)
	}
	logf(s.logger, "[DEBUG:add-member] split pane from %s → %s", splitFrom, newPaneID)

	// 2. ペインタイトル設定（失敗=warning）
	if err := s.titleSetter.SetPaneTitle(ctx, newPaneID, cmd.PaneTitle); err != nil {
		logf(s.logger, "[DEBUG:add-member] set pane title warning: %v", err)
		warnings = append(warnings, fmt.Sprintf("failed to set pane title: %v", err))
	}

	// 3. Shell初期化待ち
	if err := s.sleepFn(ctx, shellInitDelay); err != nil {
		return AddMemberResult{}, operationError(s.logger, "context cancelled during shell init wait", err)
	}

	// 4. cdコマンド送信
	if s.projectRoot != "" {
		cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(s.projectRoot, `"`, `\"`))
		if err := s.sender.SendKeys(ctx, newPaneID, cdCommand); err != nil {
			return AddMemberResult{}, operationError(s.logger, "failed to send cd command", err)
		}
		if err := s.sleepFn(ctx, cdDelay); err != nil {
			return AddMemberResult{}, operationError(s.logger, "context cancelled during cd wait", err)
		}
	}

	// 5. 起動コマンド送信
	launchCmd := buildMemberLaunchCommand(cmd.Command, cmd.Args)
	if err := s.sender.SendKeys(ctx, newPaneID, launchCmd); err != nil {
		return AddMemberResult{}, operationError(s.logger, "failed to send launch command", err)
	}

	// 6. ブートストラップ遅延
	if err := s.sleepFn(ctx, time.Duration(delayMs)*time.Millisecond); err != nil {
		return AddMemberResult{}, operationError(s.logger, "context cancelled during bootstrap wait", err)
	}

	// 7. エージェント名導出
	agentName := sanitizeMemberAgentName(cmd.PaneTitle)

	// 8. ブートストラップメッセージ構築・送信
	bootstrapMsg := buildMemberBootstrapMessage(teamName, cmd, newPaneID, agentName)
	if isClaudeCLI(cmd.Command) {
		if err := s.pasteSender.SendKeysPaste(ctx, newPaneID, bootstrapMsg); err != nil {
			logf(s.logger, "[DEBUG:add-member] bootstrap paste warning: %v", err)
			warnings = append(warnings, fmt.Sprintf("failed to send bootstrap message: %v", err))
		}
	} else {
		if err := s.sender.SendKeys(ctx, newPaneID, bootstrapMsg); err != nil {
			logf(s.logger, "[DEBUG:add-member] bootstrap send warning: %v", err)
			warnings = append(warnings, fmt.Sprintf("failed to send bootstrap message: %v", err))
		}
	}

	return AddMemberResult{
		PaneID:    newPaneID,
		PaneTitle: cmd.PaneTitle,
		AgentName: agentName,
		Warnings:  warnings,
	}, nil
}

// clampBootstrapDelay はブートストラップ遅延をデフォルト値と範囲内にクランプする。
func clampBootstrapDelay(ms int) int {
	if ms <= 0 {
		return BootstrapDelayDefault
	}
	return max(BootstrapDelayMin, min(ms, BootstrapDelayMax))
}

var memberAgentNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeMemberAgentName はペインタイトルからエージェント名を導出する。
func sanitizeMemberAgentName(paneTitle string) string {
	normalized := strings.ToLower(strings.TrimSpace(paneTitle))
	normalized = memberAgentNameSanitizer.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "member"
	}
	return normalized
}

// isClaudeCLI はコマンドが Claude CLI か判定する。
func isClaudeCLI(command string) bool {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(parts[0]))
	return base == "claude" || base == "claude.exe" || base == "claude.cmd" || strings.HasPrefix(base, "claude-code")
}

// buildMemberLaunchCommand は起動コマンド文字列を構築する。
func buildMemberLaunchCommand(command string, args []string) string {
	parts := []string{strings.TrimSpace(command)}
	for _, arg := range args {
		parts = append(parts, quoteMemberCommandArg(arg))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// quoteMemberCommandArg はコマンド引数をクォートする。
func quoteMemberCommandArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if !strings.ContainsAny(arg, " \t\"") {
		return arg
	}
	var buf strings.Builder
	buf.Grow(len(arg) + 2)
	buf.WriteByte('"')
	for i := 0; i < len(arg); i++ {
		if arg[i] == '\\' {
			j := i
			for j < len(arg) && arg[j] == '\\' {
				j++
			}
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

// buildMemberBootstrapMessage はブートストラップメッセージを構築する。
func buildMemberBootstrapMessage(teamName string, cmd AddMemberCmd, paneID, agentName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "あなたは「%s」チームのメンバーです。\n", strings.TrimSpace(teamName))
	fmt.Fprintf(&b, "役割名: %s\n", cmd.Role)
	if cmd.CustomMessage != "" {
		b.WriteString("\n")
		b.WriteString(cmd.CustomMessage)
		b.WriteString("\n")
	}
	if len(cmd.Skills) > 0 {
		b.WriteString("\n得意分野:\n")
		for _, skill := range cmd.Skills {
			if skill.Description != "" {
				fmt.Fprintf(&b, "- %s: %s\n", skill.Name, skill.Description)
			} else {
				fmt.Fprintf(&b, "- %s\n", skill.Name)
			}
		}
	}
	b.WriteString("\n--- エージェント登録 ---\n")
	b.WriteString("自身のペインIDは環境変数 $TMUX_PANE で確認できます。\n")
	fmt.Fprintf(&b, "現在のペインID: %s\n", paneID)
	b.WriteString("まず以下を実行して自身をオーケストレーターに登録してください:\n")
	escapedRole := strings.NewReplacer(`"`, `\"`, "\n", " ").Replace(cmd.Role)
	if len(cmd.Skills) > 0 {
		skillsJSON, err := json.Marshal(cmd.Skills)
		if err != nil {
			skillsJSON = []byte("[]")
		}
		fmt.Fprintf(&b, "register_agent(name=\"%s\", pane_id=\"%s\", role=\"%s\", skills=%s)\n", agentName, paneID, escapedRole, string(skillsJSON))
	} else {
		fmt.Fprintf(&b, "register_agent(name=\"%s\", pane_id=\"%s\", role=\"%s\")\n", agentName, paneID, escapedRole)
	}

	b.WriteString("\n--- ワークフロー ---\n")
	b.WriteString("1. register_agent → 自身を登録（必須・最初に実行）\n")
	b.WriteString("2. list_agents → チームメンバーとペイン状態を確認\n")
	b.WriteString("3. send_task → 他エージェントにタスクを依頼（from_agent=自分の名前）\n")
	b.WriteString("4. get_my_tasks → 自分宛タスクを確認（デフォルト: pending のみ）\n")
	b.WriteString("5. send_response → タスクに返信し completed に更新（task_id 必須）\n")
	b.WriteString("\nタスク状態: pending → completed / failed / abandoned\n")
	b.WriteString("確認: check_tasks で全タスク一覧、capture_pane で相手の画面を取得\n")

	return b.String()
}
