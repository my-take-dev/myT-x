package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	sample_teams "myT-x/embed/sample_teams"
	"myT-x/internal/config"
	"myT-x/internal/tmux"

	"github.com/google/uuid"
)

const (
	orchestratorTeamDefinitionsFileName = "orchestrator-team-definitions.json"
	orchestratorTeamMembersFileName     = "orchestrator-team-members.json"

	orchestratorLaunchModeActiveSession = "active_session"
	orchestratorLaunchModeNewSession    = "new_session"
	orchestratorTeamShellInitDelay      = 500 * time.Millisecond
	orchestratorTeamCdDelay             = 300 * time.Millisecond
	orchestratorTeamBootstrapDelay      = 3 * time.Second
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

type OrchestratorTeamDefinition struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description,omitempty"`
	Order            int                      `json:"order"`
	BootstrapDelayMs int                      `json:"bootstrap_delay_ms,omitempty"`
	StorageLocation  string                   `json:"storage_location,omitempty"`
	Members          []OrchestratorTeamMember `json:"members"`
}

const (
	orchestratorTeamBootstrapDelayMsDefault = 3000
	orchestratorTeamBootstrapDelayMsMin     = 1000
	orchestratorTeamBootstrapDelayMsMax     = 30000

	orchestratorStorageLocationGlobal  = "global"
	orchestratorStorageLocationProject = "project"
)

// OrchestratorTeamMemberSkill はチームメンバーの得意分野を表す。
type OrchestratorTeamMemberSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type OrchestratorTeamMember struct {
	ID            string                        `json:"id"`
	TeamID        string                        `json:"team_id"`
	Order         int                           `json:"order"`
	PaneTitle     string                        `json:"pane_title"`
	Role          string                        `json:"role"`
	Command       string                        `json:"command"`
	Args          []string                      `json:"args"`
	CustomMessage string                        `json:"custom_message"`
	Skills        []OrchestratorTeamMemberSkill `json:"skills,omitempty"`
}

type StartOrchestratorTeamRequest struct {
	TeamID            string `json:"team_id"`
	LaunchMode        string `json:"launch_mode"`
	SourceSessionName string `json:"source_session_name"`
	NewSessionName    string `json:"new_session_name"`
}

type StartOrchestratorTeamResult struct {
	SessionName   string            `json:"session_name"`
	LaunchMode    string            `json:"launch_mode"`
	MemberPaneIDs map[string]string `json:"member_pane_ids"`
	Warnings      []string          `json:"warnings"`
}

type orchestratorTeamFileRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Order            int    `json:"order"`
	BootstrapDelayMs int    `json:"bootstrap_delay_ms,omitempty"`
}

func (t *OrchestratorTeamDefinition) Normalize() {
	if t == nil {
		return
	}
	t.ID = strings.TrimSpace(t.ID)
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	t.Name = strings.TrimSpace(t.Name)
	t.Description = strings.TrimSpace(t.Description)
	if t.BootstrapDelayMs <= 0 {
		t.BootstrapDelayMs = orchestratorTeamBootstrapDelayMsDefault
	}
	members := make([]OrchestratorTeamMember, 0, len(t.Members))
	for index, member := range t.Members {
		member.Normalize()
		member.TeamID = t.ID
		member.Order = index
		members = append(members, member)
	}
	t.Members = members
}

func (t *OrchestratorTeamDefinition) Validate() error {
	if t == nil {
		return errors.New("team is required")
	}
	if strings.TrimSpace(t.ID) == "" {
		return errors.New("team id is required")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("team name is required")
	}
	if len([]rune(t.Description)) > 400 {
		return errors.New("team description must be 400 characters or fewer")
	}
	if t.BootstrapDelayMs < orchestratorTeamBootstrapDelayMsMin || t.BootstrapDelayMs > orchestratorTeamBootstrapDelayMsMax {
		return fmt.Errorf("bootstrap_delay_ms must be between %d and %d", orchestratorTeamBootstrapDelayMsMin, orchestratorTeamBootstrapDelayMsMax)
	}
	memberIDs := make(map[string]struct{}, len(t.Members))
	paneTitles := make(map[string]struct{}, len(t.Members))
	for _, member := range t.Members {
		if member.TeamID != t.ID {
			return fmt.Errorf("member %s team id mismatch", member.ID)
		}
		if err := member.Validate(); err != nil {
			return err
		}
		if _, exists := memberIDs[member.ID]; exists {
			return fmt.Errorf("duplicate member id: %s", member.ID)
		}
		memberIDs[member.ID] = struct{}{}
		title := strings.TrimSpace(member.PaneTitle)
		if title != "" {
			if _, exists := paneTitles[title]; exists {
				return fmt.Errorf("duplicate pane title: %s", title)
			}
			paneTitles[title] = struct{}{}
		}
	}
	return nil
}

func (m *OrchestratorTeamMember) Normalize() {
	if m == nil {
		return
	}
	m.ID = strings.TrimSpace(m.ID)
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.TeamID = strings.TrimSpace(m.TeamID)
	m.PaneTitle = strings.TrimSpace(m.PaneTitle)
	m.Role = strings.TrimSpace(m.Role)
	m.Command = strings.TrimSpace(m.Command)
	args := make([]string, 0, len(m.Args))
	for _, arg := range m.Args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		args = append(args, trimmed)
	}
	m.Args = args
	m.CustomMessage = strings.TrimSpace(m.CustomMessage)
	// スキルの正規化: 空名をフィルタ、トリム
	skills := make([]OrchestratorTeamMemberSkill, 0, len(m.Skills))
	for _, s := range m.Skills {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		skills = append(skills, OrchestratorTeamMemberSkill{
			Name:        name,
			Description: strings.TrimSpace(s.Description),
		})
	}
	m.Skills = skills
	if m.Order < 0 {
		m.Order = 0
	}
}

func (m *OrchestratorTeamMember) Validate() error {
	if m == nil {
		return errors.New("member is required")
	}
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("member id is required")
	}
	if strings.TrimSpace(m.TeamID) == "" {
		return errors.New("member team id is required")
	}
	if strings.TrimSpace(m.PaneTitle) == "" {
		return errors.New("member pane title is required")
	}
	if len([]rune(m.PaneTitle)) > 30 {
		return fmt.Errorf("member pane title must be 30 characters or fewer")
	}
	if strings.TrimSpace(m.Role) == "" {
		return fmt.Errorf("member %s role is required", m.PaneTitle)
	}
	if len([]rune(m.Role)) > 50 {
		return fmt.Errorf("member %s role must be 50 characters or fewer", m.PaneTitle)
	}
	if strings.TrimSpace(m.Command) == "" {
		return fmt.Errorf("member %s command is required", m.PaneTitle)
	}
	if len([]rune(m.Command)) > 100 {
		return fmt.Errorf("member %s command must be 100 characters or fewer", m.PaneTitle)
	}
	if len(m.Skills) > 20 {
		return fmt.Errorf("member %s skills must be 20 or fewer", m.PaneTitle)
	}
	for i, s := range m.Skills {
		if len([]rune(s.Name)) > 100 {
			return fmt.Errorf("member %s skills[%d] name must be 100 characters or fewer", m.PaneTitle, i)
		}
		if len([]rune(s.Description)) > 400 {
			return fmt.Errorf("member %s skills[%d] description must be 400 characters or fewer", m.PaneTitle, i)
		}
	}
	return nil
}

func (r *StartOrchestratorTeamRequest) Normalize() {
	if r == nil {
		return
	}
	r.TeamID = strings.TrimSpace(r.TeamID)
	r.LaunchMode = strings.TrimSpace(r.LaunchMode)
	if r.LaunchMode == "" {
		r.LaunchMode = orchestratorLaunchModeActiveSession
	}
	r.SourceSessionName = strings.TrimSpace(r.SourceSessionName)
	r.NewSessionName = strings.TrimSpace(r.NewSessionName)
}

func (r *StartOrchestratorTeamRequest) Validate() error {
	if r == nil {
		return errors.New("request is required")
	}
	if r.TeamID == "" {
		return errors.New("team id is required")
	}
	switch r.LaunchMode {
	case orchestratorLaunchModeActiveSession:
	case orchestratorLaunchModeNewSession:
	default:
		return fmt.Errorf("unsupported launch mode: %s", r.LaunchMode)
	}
	return nil
}

func (a *App) SaveOrchestratorTeam(team OrchestratorTeamDefinition, sessionName string) error {
	storageLocation := strings.TrimSpace(team.StorageLocation)
	team.StorageLocation = "" // Do not persist storage_location to file.
	team.Normalize()
	if err := team.Validate(); err != nil {
		return err
	}

	var definitionsPath, membersPath string
	if storageLocation == orchestratorStorageLocationProject {
		var err error
		definitionsPath, membersPath, err = a.resolveOrchestratorTeamProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = a.resolveOrchestratorTeamStoragePaths()
	}

	a.orchestratorTeamMu.Lock()
	defer a.orchestratorTeamMu.Unlock()

	definitions, err := readOrchestratorTeamDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	members, err := readOrchestratorTeamMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	for _, d := range definitions {
		if d.ID != team.ID && strings.TrimSpace(d.Name) == strings.TrimSpace(team.Name) {
			return fmt.Errorf("team name %q already exists", team.Name)
		}
	}

	definitions = upsertOrchestratorTeamDefinition(definitions, team)
	members = upsertOrchestratorTeamMembers(members, team)
	members = filterOrchestratorOrphanMembers(members, definitions)
	sortOrchestratorTeamDefinitions(definitions)
	sortOrchestratorTeamMembers(members)

	if err := writeOrchestratorTeamMembers(membersPath, members); err != nil {
		return err
	}
	if err := writeOrchestratorTeamDefinitions(definitionsPath, definitions); err != nil {
		return err
	}
	return nil
}

func (a *App) LoadOrchestratorTeams(sessionName string) ([]OrchestratorTeamDefinition, error) {
	definitionsPath, membersPath := a.resolveOrchestratorTeamStoragePaths()

	a.orchestratorTeamMu.Lock()
	defer a.orchestratorTeamMu.Unlock()

	definitions, err := readOrchestratorTeamDefinitions(definitionsPath)
	if err != nil {
		return []OrchestratorTeamDefinition{}, fmt.Errorf("read team definitions: %w", err)
	}

	// グローバル定義が空 → サンプルシード試行
	if len(definitions) == 0 {
		if seeded := seedOrchestratorTeamSamples(definitionsPath, membersPath); seeded {
			definitions, _ = readOrchestratorTeamDefinitions(definitionsPath)
		}
	}

	members, err := readOrchestratorTeamMembers(membersPath)
	if err != nil {
		return []OrchestratorTeamDefinition{}, fmt.Errorf("read team members: %w", err)
	}
	globalTeams := joinOrchestratorTeams(definitions, members)
	for i := range globalTeams {
		globalTeams[i].StorageLocation = orchestratorStorageLocationGlobal
	}

	// Merge project-local teams if session is specified.
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return globalTeams, nil
	}
	projDefPath, projMemPath, err := a.resolveOrchestratorTeamProjectStoragePaths(sessionName)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project storage path resolution skipped", "error", err)
		return globalTeams, nil
	}
	projDefs, err := readOrchestratorTeamDefinitions(projDefPath)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project team definitions not available", "error", err)
		return globalTeams, nil
	}
	projMembers, err := readOrchestratorTeamMembers(projMemPath)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project team members not available", "error", err)
		projMembers = []OrchestratorTeamMember{}
	}
	projectTeams := joinOrchestratorTeams(projDefs, projMembers)
	for i := range projectTeams {
		projectTeams[i].StorageLocation = orchestratorStorageLocationProject
	}

	return append(globalTeams, projectTeams...), nil
}

func (a *App) DeleteOrchestratorTeam(teamID string, storageLocation string, sessionName string) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return errors.New("team id is required")
	}

	var definitionsPath, membersPath string
	if strings.TrimSpace(storageLocation) == orchestratorStorageLocationProject {
		var err error
		definitionsPath, membersPath, err = a.resolveOrchestratorTeamProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = a.resolveOrchestratorTeamStoragePaths()
	}

	a.orchestratorTeamMu.Lock()
	defer a.orchestratorTeamMu.Unlock()

	definitions, err := readOrchestratorTeamDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	members, err := readOrchestratorTeamMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	filteredDefinitions := make([]orchestratorTeamFileRecord, 0, len(definitions))
	removed := false
	for _, definition := range definitions {
		if definition.ID == teamID {
			removed = true
			continue
		}
		filteredDefinitions = append(filteredDefinitions, definition)
	}
	if !removed {
		return fmt.Errorf("team %s not found", teamID)
	}

	filteredMembers := make([]OrchestratorTeamMember, 0, len(members))
	for _, member := range members {
		if member.TeamID == teamID {
			continue
		}
		filteredMembers = append(filteredMembers, member)
	}
	filteredMembers = filterOrchestratorOrphanMembers(filteredMembers, filteredDefinitions)
	sortOrchestratorTeamDefinitions(filteredDefinitions)
	sortOrchestratorTeamMembers(filteredMembers)

	if err := writeOrchestratorTeamDefinitions(definitionsPath, filteredDefinitions); err != nil {
		return err
	}
	if err := writeOrchestratorTeamMembers(membersPath, filteredMembers); err != nil {
		return err
	}
	return nil
}

func (a *App) ReorderOrchestratorTeams(teamIDs []string, storageLocation string, sessionName string) error {
	if len(teamIDs) == 0 {
		return nil
	}

	var definitionsPath string
	if strings.TrimSpace(storageLocation) == orchestratorStorageLocationProject {
		var err error
		definitionsPath, _, err = a.resolveOrchestratorTeamProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, _ = a.resolveOrchestratorTeamStoragePaths()
	}

	a.orchestratorTeamMu.Lock()
	defer a.orchestratorTeamMu.Unlock()

	definitions, err := readOrchestratorTeamDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}

	byID := make(map[string]*orchestratorTeamFileRecord, len(definitions))
	for i := range definitions {
		byID[definitions[i].ID] = &definitions[i]
	}

	for i, id := range teamIDs {
		record, exists := byID[id]
		if !exists {
			return fmt.Errorf("team %s not found", id)
		}
		record.Order = i
	}

	sortOrchestratorTeamDefinitions(definitions)

	if err := writeOrchestratorTeamDefinitions(definitionsPath, definitions); err != nil {
		return err
	}
	return nil
}

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
				slog.Warn("[DEBUG-ORCH-TEAM] failed to rollback new session after target preparation failure",
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
				slog.Warn("[DEBUG-ORCH-TEAM] failed to rollback new session after pre-launch failure",
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
			cdCommand := fmt.Sprintf("cd '%s'", sourceRootPath)
			slog.Info("[DEBUG-SENDKEYS] cd command", "paneID", paneID, "member", member.PaneTitle, "fullText", cdCommand)
			if err := sendKeysForOrchestratorTeamFn(router, paneID, cdCommand); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to cd for member %s in pane %s: %v", member.PaneTitle, paneID, err))
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
		slog.Warn("[DEBUG-ORCH-TEAM] launch completed with warnings",
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
			return "", nil, true, nil, fmt.Errorf("session %s has no active window", createdSession.Name)
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
	lower := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(lower, "claude")
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
	escaped := strings.ReplaceAll(arg, `"`, `\"`)
	return `"` + escaped + `"`
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
		skillsJSON, _ := json.Marshal(member.Skills)
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

func (a *App) resolveOrchestratorTeamStoragePaths() (string, string) {
	configPath := strings.TrimSpace(a.configPath)
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	baseDir := filepath.Dir(configPath)
	return filepath.Join(baseDir, orchestratorTeamDefinitionsFileName), filepath.Join(baseDir, orchestratorTeamMembersFileName)
}

// resolveOrchestratorTeamProjectStoragePaths resolves the team definition/member
// file paths inside the .myT-x directory of the given session's working directory.
func (a *App) resolveOrchestratorTeamProjectStoragePaths(sessionName string) (string, string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", "", errors.New("session name is required for project storage")
	}
	snapshot, err := a.findSessionSnapshotByName(sessionName)
	if err != nil {
		return "", "", err
	}
	rootPath, err := resolveOrchestratorSourceRootPath(snapshot)
	if err != nil {
		return "", "", err
	}
	baseDir := filepath.Join(rootPath, ".myT-x")
	return filepath.Join(baseDir, orchestratorTeamDefinitionsFileName),
		filepath.Join(baseDir, orchestratorTeamMembersFileName), nil
}

func upsertOrchestratorTeamDefinition(definitions []orchestratorTeamFileRecord, team OrchestratorTeamDefinition) []orchestratorTeamFileRecord {
	found := false
	for index, definition := range definitions {
		if definition.ID != team.ID {
			continue
		}
		definitions[index].Name = team.Name
		definitions[index].Description = team.Description
		definitions[index].BootstrapDelayMs = team.BootstrapDelayMs
		found = true
		break
	}
	if !found {
		definitions = append(definitions, orchestratorTeamFileRecord{
			ID:               team.ID,
			Name:             team.Name,
			Description:      team.Description,
			Order:            len(definitions),
			BootstrapDelayMs: team.BootstrapDelayMs,
		})
	}
	return definitions
}

func upsertOrchestratorTeamMembers(existing []OrchestratorTeamMember, team OrchestratorTeamDefinition) []OrchestratorTeamMember {
	filtered := make([]OrchestratorTeamMember, 0, len(existing)+len(team.Members))
	for _, member := range existing {
		if member.TeamID == team.ID {
			continue
		}
		filtered = append(filtered, member)
	}
	filtered = append(filtered, team.Members...)
	return filtered
}

func filterOrchestratorOrphanMembers(members []OrchestratorTeamMember, definitions []orchestratorTeamFileRecord) []OrchestratorTeamMember {
	if len(members) == 0 {
		return []OrchestratorTeamMember{}
	}
	teamIDs := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		teamIDs[definition.ID] = struct{}{}
	}
	filtered := make([]OrchestratorTeamMember, 0, len(members))
	for _, member := range members {
		if _, exists := teamIDs[member.TeamID]; !exists {
			continue
		}
		filtered = append(filtered, member)
	}
	return filtered
}

func joinOrchestratorTeams(definitions []orchestratorTeamFileRecord, members []OrchestratorTeamMember) []OrchestratorTeamDefinition {
	joined := make([]OrchestratorTeamDefinition, 0, len(definitions))
	membersByTeam := make(map[string][]OrchestratorTeamMember, len(definitions))
	for _, member := range members {
		membersByTeam[member.TeamID] = append(membersByTeam[member.TeamID], member)
	}
	for _, definition := range definitions {
		teamMembers := append([]OrchestratorTeamMember{}, membersByTeam[definition.ID]...)
		sortOrchestratorTeamMembers(teamMembers)
		delayMs := definition.BootstrapDelayMs
		if delayMs <= 0 {
			delayMs = orchestratorTeamBootstrapDelayMsDefault
		}
		joined = append(joined, OrchestratorTeamDefinition{
			ID:               definition.ID,
			Name:             definition.Name,
			Description:      definition.Description,
			Order:            definition.Order,
			BootstrapDelayMs: delayMs,
			Members:          teamMembers,
		})
	}
	sort.SliceStable(joined, func(i, j int) bool {
		if joined[i].Order != joined[j].Order {
			return joined[i].Order < joined[j].Order
		}
		left := strings.ToLower(joined[i].Name)
		right := strings.ToLower(joined[j].Name)
		if left != right {
			return left < right
		}
		return joined[i].ID < joined[j].ID
	})
	return joined
}

func sortOrchestratorTeamDefinitions(definitions []orchestratorTeamFileRecord) {
	sort.SliceStable(definitions, func(i, j int) bool {
		if definitions[i].Order != definitions[j].Order {
			return definitions[i].Order < definitions[j].Order
		}
		left := strings.ToLower(definitions[i].Name)
		right := strings.ToLower(definitions[j].Name)
		if left != right {
			return left < right
		}
		return definitions[i].ID < definitions[j].ID
	})
}

func sortOrchestratorTeamMembers(members []OrchestratorTeamMember) {
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].TeamID != members[j].TeamID {
			return members[i].TeamID < members[j].TeamID
		}
		if members[i].Order != members[j].Order {
			return members[i].Order < members[j].Order
		}
		left := strings.ToLower(members[i].PaneTitle)
		right := strings.ToLower(members[j].PaneTitle)
		if left != right {
			return left < right
		}
		return members[i].ID < members[j].ID
	})
}

func readOrchestratorTeamDefinitions(path string) ([]orchestratorTeamFileRecord, error) {
	return readOrchestratorTeamDefinitionsWithMode(path, true)
}

func readOrchestratorTeamDefinitionsForWrite(path string) ([]orchestratorTeamFileRecord, error) {
	return readOrchestratorTeamDefinitionsWithMode(path, false)
}

func readOrchestratorTeamDefinitionsWithMode(path string, allowMalformed bool) ([]orchestratorTeamFileRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []orchestratorTeamFileRecord{}, nil
		}
		return nil, err
	}

	var definitions []orchestratorTeamFileRecord
	if err := json.Unmarshal(data, &definitions); err != nil {
		if allowMalformed {
			slog.Warn("[DEBUG-ORCH-TEAM] failed to parse team definitions, returning empty", "path", path, "error", err)
			return []orchestratorTeamFileRecord{}, nil
		}
		slog.Warn("[DEBUG-ORCH-TEAM] failed to parse team definitions, refusing to overwrite", "path", path, "error", err)
		return nil, fmt.Errorf("parse team definitions: %w", err)
	}
	for index := range definitions {
		definitions[index].ID = strings.TrimSpace(definitions[index].ID)
		definitions[index].Name = strings.TrimSpace(definitions[index].Name)
	}
	return definitions, nil
}

func readOrchestratorTeamMembers(path string) ([]OrchestratorTeamMember, error) {
	return readOrchestratorTeamMembersWithMode(path, true)
}

func readOrchestratorTeamMembersForWrite(path string) ([]OrchestratorTeamMember, error) {
	return readOrchestratorTeamMembersWithMode(path, false)
}

func readOrchestratorTeamMembersWithMode(path string, allowMalformed bool) ([]OrchestratorTeamMember, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []OrchestratorTeamMember{}, nil
		}
		return nil, err
	}

	var members []OrchestratorTeamMember
	if err := json.Unmarshal(data, &members); err != nil {
		if allowMalformed {
			slog.Warn("[DEBUG-ORCH-TEAM] failed to parse team members, returning empty", "path", path, "error", err)
			return []OrchestratorTeamMember{}, nil
		}
		slog.Warn("[DEBUG-ORCH-TEAM] failed to parse team members, refusing to overwrite", "path", path, "error", err)
		return nil, fmt.Errorf("parse team members: %w", err)
	}
	for index := range members {
		members[index].Normalize()
	}
	return members, nil
}

func writeOrchestratorTeamDefinitions(path string, definitions []orchestratorTeamFileRecord) error {
	return writeOrchestratorTeamJSON(path, definitions, "team definitions")
}

func writeOrchestratorTeamMembers(path string, members []OrchestratorTeamMember) error {
	return writeOrchestratorTeamJSON(path, members, "team members")
}

func writeOrchestratorTeamJSON(path string, payload any, label string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create orchestrator team directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", label, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", label, err)
	}
	return nil
}

// seedOrchestratorTeamSamples は定義ファイルが未存在の場合に埋め込みサンプルを書き出す。
// ユーザーが全チーム削除後（ファイルは存在するが空配列）には再シードしない。
func seedOrchestratorTeamSamples(definitionsPath, membersPath string) bool {
	// ファイルが既に存在する場合はシードしない
	if _, err := os.Stat(definitionsPath); err == nil {
		return false
	}

	defsData, err := sample_teams.FS.ReadFile(orchestratorTeamDefinitionsFileName)
	if err != nil {
		slog.Warn("[DEBUG-ORCH-TEAM] failed to read embedded sample definitions", "error", err)
		return false
	}
	membersData, err := sample_teams.FS.ReadFile(orchestratorTeamMembersFileName)
	if err != nil {
		slog.Warn("[DEBUG-ORCH-TEAM] failed to read embedded sample members", "error", err)
		return false
	}

	dir := filepath.Dir(definitionsPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("[DEBUG-ORCH-TEAM] failed to create sample team directory", "error", err)
		return false
	}
	if err := os.WriteFile(definitionsPath, defsData, 0o644); err != nil {
		slog.Warn("[DEBUG-ORCH-TEAM] failed to write sample definitions", "error", err)
		return false
	}
	if err := os.WriteFile(membersPath, membersData, 0o644); err != nil {
		slog.Warn("[DEBUG-ORCH-TEAM] failed to write sample members", "error", err)
		return false
	}
	slog.Info("[DEBUG-ORCH-TEAM] seeded sample team definitions for first-time use")
	return true
}
