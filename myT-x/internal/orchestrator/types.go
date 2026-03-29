package orchestrator

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Bootstrap delay configuration constants.
const (
	BootstrapDelayMsDefault = 5000
	BootstrapDelayMsMin     = 1000
	BootstrapDelayMsMax     = 30000
)

// Storage location constants.
const (
	StorageLocationGlobal  = "global"
	StorageLocationProject = "project"
)

// Launch mode constants.
const (
	LaunchModeActiveSession = "active_session"
	LaunchModeNewSession    = "new_session"
)

// Unaffiliated (system) team constants.
const (
	UnaffiliatedTeamID          = "__unaffiliated__"
	UnaffiliatedTeamName        = "無所属"
	UnaffiliatedTeamDescription = "手動追加メンバーの保存先（システムチーム）"
	UnaffiliatedTeamOrder       = 9999
)

// IsSystemTeam returns true if the given team ID is a system-managed team.
func IsSystemTeam(teamID string) bool {
	return teamID == UnaffiliatedTeamID
}

// Internal file names and timing constants.
const (
	definitionsFileName        = "orchestrator-team-definitions.json"
	membersFileName            = "orchestrator-team-members.json"
	shellInitDelay             = 500 * time.Millisecond
	cdDelay                    = 300 * time.Millisecond
	bootstrapInterMessageDelay = 300 * time.Millisecond // inter-message delay between sequential bootstrap injections
	renameRetryMax             = 10
	renameRetryBaseDelay       = 10 * time.Millisecond
)

// teamFileRecord is the on-disk representation of a team definition header.
// Members are stored separately for easier partial updates.
type teamFileRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Order            int    `json:"order"`
	BootstrapDelayMs int    `json:"bootstrap_delay_ms,omitempty"`
}

// TeamDefinition is the complete definition of an orchestrator team.
type TeamDefinition struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Description      string       `json:"description,omitempty"`
	Order            int          `json:"order"`
	BootstrapDelayMs int          `json:"bootstrap_delay_ms,omitempty"`
	StorageLocation  string       `json:"storage_location,omitempty"`
	Members          []TeamMember `json:"members"`
}

// TeamMemberSkill is a member's area of expertise.
type TeamMemberSkill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// TeamMember is the configuration of one team member.
type TeamMember struct {
	ID            string            `json:"id"`
	TeamID        string            `json:"team_id"`
	Order         int               `json:"order"`
	PaneTitle     string            `json:"pane_title"`
	Role          string            `json:"role"`
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	CustomMessage string            `json:"custom_message"`
	Skills        []TeamMemberSkill `json:"skills,omitempty"`
}

// StartTeamRequest is the frontend request to launch a team.
type StartTeamRequest struct {
	TeamID            string `json:"team_id"`
	LaunchMode        string `json:"launch_mode"`
	SourceSessionName string `json:"source_session_name"`
	NewSessionName    string `json:"new_session_name"`
}

// StartTeamResult is the result of launching a team.
type StartTeamResult struct {
	SessionName   string            `json:"session_name"`
	LaunchMode    string            `json:"launch_mode"`
	MemberPaneIDs map[string]string `json:"member_pane_ids"`
	Warnings      []string          `json:"warnings"`
}

// Normalize normalizes team definition fields.
func (t *TeamDefinition) Normalize() {
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
		t.BootstrapDelayMs = BootstrapDelayMsDefault
	}
	members := make([]TeamMember, 0, len(t.Members))
	for index, member := range t.Members {
		member.Normalize()
		member.TeamID = t.ID
		member.Order = index
		members = append(members, member)
	}
	t.Members = members
}

// Validate validates team definition fields.
func (t *TeamDefinition) Validate() error {
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
	if t.BootstrapDelayMs < BootstrapDelayMsMin || t.BootstrapDelayMs > BootstrapDelayMsMax {
		return fmt.Errorf("bootstrap_delay_ms must be between %d and %d", BootstrapDelayMsMin, BootstrapDelayMsMax)
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

// Normalize normalizes member fields.
func (m *TeamMember) Normalize() {
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
	skills := make([]TeamMemberSkill, 0, len(m.Skills))
	for _, s := range m.Skills {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		skills = append(skills, TeamMemberSkill{
			Name:        name,
			Description: strings.TrimSpace(s.Description),
		})
	}
	m.Skills = skills
	if m.Order < 0 {
		m.Order = 0
	}
}

// Validate validates member fields.
func (m *TeamMember) Validate() error {
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

// Normalize normalizes start request fields.
func (r *StartTeamRequest) Normalize() {
	if r == nil {
		return
	}
	r.TeamID = strings.TrimSpace(r.TeamID)
	r.LaunchMode = strings.TrimSpace(r.LaunchMode)
	if r.LaunchMode == "" {
		r.LaunchMode = LaunchModeActiveSession
	}
	r.SourceSessionName = strings.TrimSpace(r.SourceSessionName)
	r.NewSessionName = strings.TrimSpace(r.NewSessionName)
}

// Validate validates start request fields.
func (r *StartTeamRequest) Validate() error {
	if r == nil {
		return errors.New("request is required")
	}
	if r.TeamID == "" {
		return errors.New("team id is required")
	}
	switch r.LaunchMode {
	case LaunchModeActiveSession:
	case LaunchModeNewSession:
	default:
		return fmt.Errorf("unsupported launch mode: %s", r.LaunchMode)
	}
	return nil
}
