package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	orchestratorTeamBootstrapDelayMsDefault = 3000
	orchestratorTeamBootstrapDelayMsMin     = 1000
	orchestratorTeamBootstrapDelayMsMax     = 30000
)

// StorageLocation はチーム定義の保存先を表す。
type StorageLocation = string

const (
	orchestratorStorageLocationGlobal  StorageLocation = "global"
	orchestratorStorageLocationProject StorageLocation = "project"
)

// LaunchMode はチーム起動モードを表す。
type LaunchMode = string

// OrchestratorTeamDefinition はオーケストレーターチームの定義情報を表す。
type OrchestratorTeamDefinition struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Description      string                   `json:"description,omitempty"`
	Order            int                      `json:"order"`
	BootstrapDelayMs int                      `json:"bootstrap_delay_ms,omitempty"`
	StorageLocation  string                   `json:"storage_location,omitempty"`
	Members          []OrchestratorTeamMember `json:"members"`
}

// OrchestratorTeamMemberSkill はチームメンバーの得意分野を表す。
type OrchestratorTeamMemberSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// OrchestratorTeamMember はチームメンバーの構成情報を表す。
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

// StartOrchestratorTeamRequest はチーム起動リクエストを表す。
type StartOrchestratorTeamRequest struct {
	TeamID            string `json:"team_id"`
	LaunchMode        string `json:"launch_mode"`
	SourceSessionName string `json:"source_session_name"`
	NewSessionName    string `json:"new_session_name"`
}

// StartOrchestratorTeamResult はチーム起動結果を表す。
type StartOrchestratorTeamResult struct {
	SessionName   string            `json:"session_name"`
	LaunchMode    string            `json:"launch_mode"`
	MemberPaneIDs map[string]string `json:"member_pane_ids"`
	Warnings      []string          `json:"warnings"`
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
