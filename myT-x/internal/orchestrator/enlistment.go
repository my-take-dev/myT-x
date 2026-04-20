package orchestrator

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SessionEnlistmentContext contains the data required to guide pane enlistment.
type SessionEnlistmentContext struct {
	Teams               []TeamDefinition  `json:"teams"`
	UnaffiliatedMembers []TeamMember      `json:"unaffiliated_members"`
	RoleCatalog         []string          `json:"role_catalog"`
	SkillCatalog        []TeamMemberSkill `json:"skill_catalog"`
	RegisteredPaneIDs   []string          `json:"registered_pane_ids,omitempty"`
}

// EnlistPaneRequest registers an existing pane as an orchestrator member target.
type EnlistPaneRequest struct {
	SessionName      string     `json:"session_name"`
	PaneID           string     `json:"pane_id"`
	TeamID           string     `json:"team_id"`
	StorageLocation  string     `json:"storage_location,omitempty"`
	PaneState        string     `json:"pane_state"`
	BootstrapDelayMs int        `json:"bootstrap_delay_ms"`
	Member           TeamMember `json:"member"`
}

// EnlistPaneResult contains non-fatal warnings produced during enlistment.
type EnlistPaneResult struct {
	Warnings []string `json:"warnings"`
}

// Normalize trims enlistment fields and normalizes the embedded member payload.
func (r *EnlistPaneRequest) Normalize() {
	if r == nil {
		return
	}
	r.SessionName = strings.TrimSpace(r.SessionName)
	r.PaneID = strings.TrimSpace(r.PaneID)
	r.TeamID = strings.TrimSpace(r.TeamID)
	r.StorageLocation = strings.TrimSpace(r.StorageLocation)
	if r.StorageLocation == "" {
		r.StorageLocation = StorageLocationGlobal
	}
	r.PaneState = strings.TrimSpace(r.PaneState)
	r.Member.Normalize()
	r.Member.TeamID = r.TeamID
	if r.BootstrapDelayMs <= 0 {
		r.BootstrapDelayMs = BootstrapDelayMsDefault
	}
	r.BootstrapDelayMs = max(BootstrapDelayMsMin, min(r.BootstrapDelayMs, BootstrapDelayMsMax))
}

// Validate verifies that the enlistment request is usable.
func (r *EnlistPaneRequest) Validate() error {
	if r == nil {
		return errors.New("request is required")
	}
	if strings.TrimSpace(r.SessionName) == "" {
		return errors.New("session_name is required")
	}
	if strings.TrimSpace(r.PaneID) == "" {
		return errors.New("pane_id is required")
	}
	if strings.TrimSpace(r.TeamID) == "" {
		return errors.New("team_id is required")
	}
	switch r.PaneState {
	case PaneStateCLIRunning, PaneStateCLINotRunning:
	default:
		return fmt.Errorf("unsupported pane_state: %q", r.PaneState)
	}
	if err := r.Member.Validate(); err != nil {
		return fmt.Errorf("member validation failed: %w", err)
	}
	return nil
}

// GetSessionEnlistmentContext aggregates saved team data for frontend enlistment UI.
func (s *Service) GetSessionEnlistmentContext(sessionName string) (SessionEnlistmentContext, error) {
	teams, err := s.LoadTeams(sessionName)
	if err != nil {
		return SessionEnlistmentContext{}, err
	}

	context := SessionEnlistmentContext{
		Teams:               teams,
		UnaffiliatedMembers: []TeamMember{},
		RoleCatalog:         []string{},
		SkillCatalog:        []TeamMemberSkill{},
		RegisteredPaneIDs:   []string{},
	}

	roleSet := make(map[string]struct{})
	skillIndexByName := make(map[string]int)

	for _, team := range teams {
		if IsSystemTeam(team.ID) {
			context.UnaffiliatedMembers = append(context.UnaffiliatedMembers, team.Members...)
		}
		for _, member := range team.Members {
			role := strings.TrimSpace(member.Role)
			if role != "" {
				if _, exists := roleSet[role]; !exists {
					roleSet[role] = struct{}{}
					context.RoleCatalog = append(context.RoleCatalog, role)
				}
			}
			for _, skill := range member.Skills {
				name := strings.TrimSpace(skill.Name)
				if name == "" {
					continue
				}
				canonicalName := strings.ToLower(name)
				description := strings.TrimSpace(skill.Description)
				if index, exists := skillIndexByName[canonicalName]; exists {
					if context.SkillCatalog[index].Description == "" && description != "" {
						context.SkillCatalog[index].Description = description
					}
					continue
				}
				skillIndexByName[canonicalName] = len(context.SkillCatalog)
				context.SkillCatalog = append(context.SkillCatalog, TeamMemberSkill{
					Name:        name,
					Description: description,
				})
			}
		}
	}

	sort.SliceStable(context.RoleCatalog, func(i, j int) bool {
		return strings.ToLower(context.RoleCatalog[i]) < strings.ToLower(context.RoleCatalog[j])
	})
	sort.SliceStable(context.SkillCatalog, func(i, j int) bool {
		return strings.ToLower(context.SkillCatalog[i].Name) < strings.ToLower(context.SkillCatalog[j].Name)
	})

	return context, nil
}
