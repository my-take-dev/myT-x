package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	// DefaultMemberTeamName is the default team name used by add_member / add_members.
	DefaultMemberTeamName = "動的チーム"

	shellInitDelay = 500 * time.Millisecond
	cdDelay        = 300 * time.Millisecond
	// bootstrapInterMessageDelay spaces Phase 2 messages to avoid collapsing them together in the target pane.
	bootstrapInterMessageDelay = 300 * time.Millisecond

	defaultTeamName = DefaultMemberTeamName
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
	reserveMu   sync.Mutex
	reserved    map[string]struct{}
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
		reserved:    make(map[string]struct{}),
	}
}

// SetSleepFn はテスト用に sleep 関数を差し替える。
func (s *MemberBootstrapService) SetSleepFn(fn func(context.Context, time.Duration) error) {
	s.sleepFn = fn
}

func resolveMemberBootstrapCaller(
	ctx context.Context,
	resolver domain.SelfPaneResolver,
	agents domain.AgentRepository,
	logger *log.Logger,
) (domain.Agent, error) {
	caller, err := resolveCaller(ctx, resolver, agents, logger)
	if err == nil {
		return caller, nil
	}
	if !errors.Is(err, errCallerNotRegistered) {
		return domain.Agent{}, err
	}

	paneID, paneErr := resolver.GetPaneID(ctx)
	if paneErr != nil || paneID == "" {
		return domain.Agent{}, err
	}
	return domain.Agent{Name: trustedCallerName, PaneID: paneID}, nil
}

// AddMember は新メンバーをペイン分割・CLI起動・ブートストラップメッセージ送信で追加する。
func (s *MemberBootstrapService) AddMember(ctx context.Context, cmd AddMemberCmd) (AddMemberResult, error) {
	// 呼び出し元の解決と登録済みエージェント確認
	caller, err := resolveMemberBootstrapCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return AddMemberResult{}, err
	}

	splitFrom := cmd.SplitFrom
	if splitFrom == "" {
		splitFrom = caller.PaneID
		if splitFrom == "" && IsTrustedCaller(caller) {
			return AddMemberResult{}, validationError("split_from is required when caller pane is unavailable")
		}
	}

	// デフォルト値
	teamName := cmd.TeamName
	if teamName == "" {
		teamName = defaultTeamName
	}
	horizontal := cmd.SplitDirection != "vertical"
	delayMs := clampBootstrapDelay(cmd.BootstrapDelayMs)
	// 7. Reserve the agent name before creating the pane so the bootstrap
	// command stays collision-free.
	agentName, err := s.reserveMemberAgentName(ctx, cmd.PaneTitle)
	if err != nil {
		return AddMemberResult{}, err
	}
	defer s.releaseMemberAgentNames([]string{agentName})

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
		return AddMemberResult{}, operationError(s.logger, fmt.Sprintf("context cancelled during shell init wait for pane %s", newPaneID), err)
	}

	// 4. cdコマンド送信
	if s.projectRoot != "" {
		cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(s.projectRoot, `"`, `\"`))
		if err := s.sender.SendKeys(ctx, newPaneID, cdCommand); err != nil {
			return AddMemberResult{}, operationError(s.logger, fmt.Sprintf("failed to send cd command for pane %s", newPaneID), err)
		}
		if err := s.sleepFn(ctx, cdDelay); err != nil {
			return AddMemberResult{}, operationError(s.logger, fmt.Sprintf("context cancelled during cd wait for pane %s", newPaneID), err)
		}
	}

	// 5. 起動コマンド送信
	launchCmd := buildMemberLaunchCommand(cmd.Command, cmd.Args)
	if err := s.sender.SendKeys(ctx, newPaneID, launchCmd); err != nil {
		return AddMemberResult{}, operationError(s.logger, fmt.Sprintf("failed to send launch command for pane %s", newPaneID), err)
	}

	// 6. ブートストラップ遅延
	if err := s.sleepFn(ctx, time.Duration(delayMs)*time.Millisecond); err != nil {
		return AddMemberResult{}, operationError(s.logger, fmt.Sprintf("context cancelled during bootstrap wait for pane %s", newPaneID), err)
	}

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

// ── Batch member addition ──

// AddMemberBatchItemCmd は add_members の個別メンバー定義。
type AddMemberBatchItemCmd struct {
	PaneTitle     string
	Role          string
	Command       string
	Args          []string
	CustomMessage string
	Skills        []domain.Skill
}

func (cmd AddMemberBatchItemCmd) toAddMemberCmd() AddMemberCmd {
	return AddMemberCmd{
		PaneTitle:     cmd.PaneTitle,
		Role:          cmd.Role,
		Command:       cmd.Command,
		Args:          cmd.Args,
		CustomMessage: cmd.CustomMessage,
		Skills:        cmd.Skills,
	}
}

// AddMembersCmd は add_members のコマンドパラメータ。
type AddMembersCmd struct {
	Members          []AddMemberBatchItemCmd
	TeamName         string // デフォルト "動的チーム"
	SplitFrom        string // 最初の分割元ペインID
	SplitDirection   string // "horizontal" / "vertical"
	BootstrapDelayMs int    // 1000-30000
}

// AddMembersItemResult は個別メンバーの結果。
type AddMembersItemResult struct {
	PaneTitle      string
	PaneID         string
	OrphanedPaneID string
	AgentName      string
	Warnings       []string
	// Error marks the item as failed. PaneID is cleared in that case, and
	// OrphanedPaneID records a pane that was created before setup failed.
	Error string
}

// AddMembersSummary はバッチ追加の集計結果。
type AddMembersSummary struct {
	Created int
	Failed  int
}

// AddMembersResult は add_members の全体結果。
type AddMembersResult struct {
	Results []AddMembersItemResult
	Summary AddMembersSummary
}

type AddMembersPartialResultError struct {
	Result AddMembersResult
	cause  error
}

func (e *AddMembersPartialResultError) Error() string {
	return e.cause.Error()
}

func (e *AddMembersPartialResultError) Unwrap() error {
	return e.cause
}

// launchedBatchMember は Phase 1 で起動に成功したメンバーの情報を保持する。
type launchedBatchMember struct {
	index     int
	paneID    string
	agentName string
	cmd       AddMemberBatchItemCmd
}

// AddMembers authenticates the caller before entering the two-phase batch flow.
// AddMembers は複数メンバーを二段階アプローチで一括追加する。
// Phase 1: ペイン分割 + CLI 起動（高速）
// Phase 2: 一回の bootstrap delay 後にブートストラップメッセージを順次送信
func (s *MemberBootstrapService) AddMembers(ctx context.Context, cmd AddMembersCmd) (AddMembersResult, error) {
	if len(cmd.Members) == 0 {
		return AddMembersResult{}, validationError("members must contain at least 1 item")
	}

	// Phase 0: 呼び出し元認証
	caller, err := resolveMemberBootstrapCaller(ctx, s.resolver, s.agents, s.logger)
	if err != nil {
		return AddMembersResult{}, err
	}

	splitFrom := cmd.SplitFrom
	if splitFrom == "" {
		splitFrom = caller.PaneID
		if splitFrom == "" && IsTrustedCaller(caller) {
			return AddMembersResult{}, validationError("split_from is required when caller pane is unavailable")
		}
	}

	teamName := cmd.TeamName
	if teamName == "" {
		teamName = defaultTeamName
	}
	horizontal := cmd.SplitDirection != "vertical"
	delayMs := clampBootstrapDelay(cmd.BootstrapDelayMs)
	agentNames, err := s.reserveMemberAgentNames(ctx, cmd.Members)
	if err != nil {
		return AddMembersResult{}, err
	}
	defer s.releaseMemberAgentNames(agentNames)
	results := make([]AddMembersItemResult, len(cmd.Members))
	for i := range results {
		results[i].PaneTitle = cmd.Members[i].PaneTitle
		results[i].AgentName = agentNames[i]
	}
	appendDuplicatePaneTitleWarnings(results, cmd.Members)

	// Phase 1: ペイン作成 + CLI 起動（高速）
	currentSplitFrom := splitFrom
	var launched []launchedBatchMember

	for i, member := range cmd.Members {
		// 1. ペイン分割
		newPaneID, splitErr := s.splitter.SplitPane(ctx, currentSplitFrom, horizontal)
		if splitErr != nil {
			logf(s.logger, "[DEBUG-ADD-MEMBERS] split pane failed for member[%d] %s: %v", i, member.PaneTitle, splitErr)
			results[i].Error = fmt.Sprintf("failed to split pane: %v", splitErr)
			continue
		}
		logf(s.logger, "[DEBUG-ADD-MEMBERS] split pane from %s → %s for member[%d] %s", currentSplitFrom, newPaneID, i, member.PaneTitle)
		results[i].PaneID = newPaneID

		// 2. ペインタイトル設定（失敗=warning）
		if titleErr := s.titleSetter.SetPaneTitle(ctx, newPaneID, member.PaneTitle); titleErr != nil {
			logf(s.logger, "[DEBUG-ADD-MEMBERS] set pane title warning for member[%d]: %v", i, titleErr)
			results[i].Warnings = append(results[i].Warnings, fmt.Sprintf("failed to set pane title: %v", titleErr))
		}

		// 3. Shell 初期化待ち
		if sleepErr := s.sleepFn(ctx, shellInitDelay); sleepErr != nil {
			// Context cancellation makes the remaining pane-setup steps fail as
			// well, so abort the batch instead of continuing item-by-item.
			markAddMembersFailure(results, i, newPaneID, "context cancelled during shell init wait", sleepErr)
			return addMembersPartialResult(results, operationError(s.logger, "context cancelled during shell init wait", sleepErr))
		}

		// 4. cd コマンド送信
		if s.projectRoot != "" {
			cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(s.projectRoot, `"`, `\"`))
			if cdErr := s.sender.SendKeys(ctx, newPaneID, cdCommand); cdErr != nil {
				logf(s.logger, "[DEBUG-ADD-MEMBERS] cd command failed for member[%d]: %v", i, cdErr)
				markAddMembersFailure(results, i, newPaneID, "failed to send cd command", cdErr)
				continue
			}
			if sleepErr := s.sleepFn(ctx, cdDelay); sleepErr != nil {
				markAddMembersFailure(results, i, newPaneID, "context cancelled during cd wait", sleepErr)
				return addMembersPartialResult(results, operationError(s.logger, "context cancelled during cd wait", sleepErr))
			}
		}

		// 5. 起動コマンド送信
		launchCmd := buildMemberLaunchCommand(member.Command, member.Args)
		if launchErr := s.sender.SendKeys(ctx, newPaneID, launchCmd); launchErr != nil {
			logf(s.logger, "[DEBUG-ADD-MEMBERS] launch command failed for member[%d]: %v", i, launchErr)
			markAddMembersFailure(results, i, newPaneID, "failed to send launch command", launchErr)
			continue
		}

		launched = append(launched, launchedBatchMember{
			index:     i,
			paneID:    newPaneID,
			agentName: agentNames[i],
			cmd:       member,
		})
		currentSplitFrom = newPaneID
	}

	// Phase 2: ブートストラップメッセージ送信（ONE WAIT）
	if len(launched) > 0 {
		if sleepErr := s.sleepFn(ctx, time.Duration(delayMs)*time.Millisecond); sleepErr != nil {
			appendBootstrapSkippedWarning(results, launched, 0, "bootstrap was skipped because context was cancelled during bootstrap wait")
			return addMembersPartialResult(results, operationError(s.logger, "context cancelled during bootstrap wait", sleepErr))
		}

		for j, lm := range launched {
			if j > 0 {
				if sleepErr := s.sleepFn(ctx, bootstrapInterMessageDelay); sleepErr != nil {
					appendBootstrapSkippedWarning(results, launched, j, "bootstrap was skipped because context was cancelled during inter-message wait")
					return addMembersPartialResult(results, operationError(s.logger, "context cancelled during inter-message wait", sleepErr))
				}
			}

			bootstrapMsg := buildMemberBootstrapMessage(teamName, lm.cmd.toAddMemberCmd(), lm.paneID, lm.agentName)

			if isClaudeCLI(lm.cmd.Command) {
				if sendErr := s.pasteSender.SendKeysPaste(ctx, lm.paneID, bootstrapMsg); sendErr != nil {
					logf(s.logger, "[DEBUG-ADD-MEMBERS] bootstrap paste warning for member[%d]: %v", lm.index, sendErr)
					results[lm.index].Warnings = append(results[lm.index].Warnings, fmt.Sprintf("failed to send bootstrap message: %v", sendErr))
				}
			} else {
				if sendErr := s.sender.SendKeys(ctx, lm.paneID, bootstrapMsg); sendErr != nil {
					logf(s.logger, "[DEBUG-ADD-MEMBERS] bootstrap send warning for member[%d]: %v", lm.index, sendErr)
					results[lm.index].Warnings = append(results[lm.index].Warnings, fmt.Sprintf("failed to send bootstrap message: %v", sendErr))
				}
			}
		}
	}

	return buildAddMembersResult(results), nil
}

// deduplicateMemberAgentNames はバッチ内のエージェント名を重複排除する。
// 同名の場合は "name", "name-2", "name-3" のようにサフィックスを付与する。
func deduplicateMemberAgentNamesWithTaken(members []AddMemberBatchItemCmd, taken map[string]struct{}) []string {
	names := make([]string, len(members))
	reserved := make(map[string]struct{}, len(members))
	for name := range taken {
		reserved[name] = struct{}{}
	}
	for i, m := range members {
		base := sanitizeMemberAgentName(m.PaneTitle)
		candidate := base
		if _, exists := reserved[candidate]; exists {
			for suffix := 2; ; suffix++ {
				candidate = fmt.Sprintf("%s-%d", base, suffix)
				if _, exists := reserved[candidate]; !exists {
					break
				}
			}
		}
		names[i] = candidate
		reserved[candidate] = struct{}{}
	}
	return names
}

func buildAddMembersResult(results []AddMembersItemResult) AddMembersResult {
	return AddMembersResult{
		Results: results,
		Summary: summarizeAddMembersResults(results),
	}
}

func summarizeAddMembersResults(results []AddMembersItemResult) AddMembersSummary {
	summary := AddMembersSummary{}
	for _, r := range results {
		if r.Error != "" {
			summary.Failed++
			continue
		}
		summary.Created++
	}
	return summary
}

func addMembersPartialResult(results []AddMembersItemResult, cause error) (AddMembersResult, error) {
	result := buildAddMembersResult(results)
	return AddMembersResult{}, &AddMembersPartialResultError{Result: result, cause: cause}
}

func markAddMembersFailure(results []AddMembersItemResult, index int, paneID, action string, cause error) {
	results[index].PaneID = ""
	results[index].OrphanedPaneID = paneID
	results[index].Error = fmt.Sprintf("%s for pane %s: %v", action, paneID, cause)
}

func appendBootstrapSkippedWarning(results []AddMembersItemResult, launched []launchedBatchMember, start int, warning string) {
	for _, member := range launched[start:] {
		results[member.index].Warnings = append(results[member.index].Warnings, warning)
	}
}

func appendDuplicatePaneTitleWarnings(results []AddMembersItemResult, members []AddMemberBatchItemCmd) {
	counts := make(map[string]int, len(members))
	for _, member := range members {
		counts[member.PaneTitle]++
	}
	for i, member := range members {
		if counts[member.PaneTitle] > 1 {
			results[i].Warnings = append(results[i].Warnings,
				"duplicate pane_title in batch; terminal panes with the same title may be harder to distinguish")
		}
	}
}

func (s *MemberBootstrapService) reserveMemberAgentName(ctx context.Context, paneTitle string) (string, error) {
	names, err := s.reserveMemberAgentNames(ctx, []AddMemberBatchItemCmd{{PaneTitle: paneTitle}})
	if err != nil {
		return "", err
	}
	return names[0], nil
}

func (s *MemberBootstrapService) reserveMemberAgentNames(ctx context.Context, members []AddMemberBatchItemCmd) ([]string, error) {
	s.reserveMu.Lock()
	defer s.reserveMu.Unlock()

	agents, err := s.agents.ListAgents(ctx)
	if err != nil {
		return nil, operationError(s.logger, "failed to list agents for name reservation", err)
	}
	taken := make(map[string]struct{}, len(agents)+len(s.reserved))
	for _, agent := range agents {
		taken[agent.Name] = struct{}{}
	}
	for name := range s.reserved {
		taken[name] = struct{}{}
	}

	names := deduplicateMemberAgentNamesWithTaken(members, taken)
	for _, name := range names {
		s.reserved[name] = struct{}{}
	}
	return names, nil
}

func (s *MemberBootstrapService) releaseMemberAgentNames(names []string) {
	s.reserveMu.Lock()
	defer s.reserveMu.Unlock()
	for _, name := range names {
		delete(s.reserved, name)
	}
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
	b.WriteString("\nタスク状態: pending / blocked → completed / failed / abandoned / cancelled / expired\n")
	b.WriteString("確認: list_all_tasks で全タスク一覧、capture_pane で相手の画面を取得\n")

	b.WriteString("\n--- アイドル時の行動（重要） ---\n")
	b.WriteString("- タスク完了後、またはアイドル中は30-60秒ごとに: get_my_tasks を呼んで自分宛タスクを確認\n")
	b.WriteString("- 作業開始時: update_status(status=\"busy\") で状態を報告\n")
	b.WriteString("- 作業完了・待機時: update_status(status=\"idle\") で状態を報告（未ackの pending タスクがあれば再配信されます）\n")
	b.WriteString("- send_task のメッセージ配信はベストエフォートです。確実に受信するには get_my_tasks を定期確認してください\n")

	return b.String()
}
