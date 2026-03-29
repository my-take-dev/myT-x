package orchestrator

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// BootstrapMemberToPaneRequest is the request to bootstrap a single member to an existing pane.
type BootstrapMemberToPaneRequest struct {
	PaneID           string     `json:"pane_id"`
	PaneState        string     `json:"pane_state"`
	TeamName         string     `json:"team_name"`
	Member           TeamMember `json:"member"`
	BootstrapDelayMs int        `json:"bootstrap_delay_ms"`
	SessionName      string     `json:"session_name"`
}

// BootstrapMemberToPaneResult is the result of bootstrapping a member to a pane.
type BootstrapMemberToPaneResult struct {
	Warnings []string `json:"warnings"`
}

// Pane state constants for BootstrapMemberToPaneRequest.
const (
	PaneStateCLIRunning    = "cli_running"
	PaneStateCLINotRunning = "cli_not_running"
)

// Normalize normalizes the bootstrap request fields.
func (r *BootstrapMemberToPaneRequest) Normalize() {
	if r == nil {
		return
	}
	r.PaneID = strings.TrimSpace(r.PaneID)
	r.PaneState = strings.TrimSpace(r.PaneState)
	r.TeamName = strings.TrimSpace(r.TeamName)
	r.SessionName = strings.TrimSpace(r.SessionName)
	r.Member.Normalize()
	if r.BootstrapDelayMs <= 0 {
		r.BootstrapDelayMs = BootstrapDelayMsDefault
	}
	r.BootstrapDelayMs = max(BootstrapDelayMsMin, min(r.BootstrapDelayMs, BootstrapDelayMsMax))
}

// Validate validates the bootstrap request fields.
func (r *BootstrapMemberToPaneRequest) Validate() error {
	if r == nil {
		return errors.New("request is required")
	}
	if r.PaneID == "" {
		return errors.New("pane_id is required")
	}
	switch r.PaneState {
	case PaneStateCLIRunning, PaneStateCLINotRunning:
	default:
		return fmt.Errorf("unsupported pane_state: %q", r.PaneState)
	}
	if r.TeamName == "" {
		return errors.New("team_name is required")
	}
	if err := r.Member.Validate(); err != nil {
		return fmt.Errorf("member validation failed: %w", err)
	}
	return nil
}

// BootstrapMemberToPane bootstraps a single member to an existing pane.
// When PaneState is "cli_not_running", it resolves the source root path,
// sends a cd command and launches the CLI before sending the bootstrap message.
// When PaneState is "cli_running", it sends only the bootstrap message.
func (s *Service) BootstrapMemberToPane(request BootstrapMemberToPaneRequest) (BootstrapMemberToPaneResult, error) {
	if err := s.deps.CheckReady(); err != nil {
		return BootstrapMemberToPaneResult{}, fmt.Errorf("runtime not ready: %w", err)
	}

	request.Normalize()
	if err := request.Validate(); err != nil {
		return BootstrapMemberToPaneResult{}, err
	}

	result := BootstrapMemberToPaneResult{
		Warnings: []string{},
	}

	paneID := request.PaneID
	member := request.Member

	if request.PaneState == PaneStateCLINotRunning {
		sessionName := request.SessionName
		if sessionName == "" {
			sessionName = s.deps.GetActiveSessionName()
		}
		session, err := s.deps.FindSessionSnapshot(sessionName)
		if err != nil {
			return result, fmt.Errorf("failed to find session %q: %w", sessionName, err)
		}
		sourceRootPath, err := ResolveSourceRootPath(session)
		if err != nil {
			return result, fmt.Errorf("failed to resolve source root path: %w", err)
		}

		if strings.TrimSpace(sourceRootPath) != "" {
			cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(sourceRootPath, `"`, `\"`))
			slog.Debug("[DEBUG-SENDKEYS] bootstrap-member cd command", "paneID", paneID, "member", member.PaneTitle, "fullText", cdCommand)
			if err := s.deps.SendKeys(paneID, cdCommand); err != nil {
				return result, fmt.Errorf("cd failed for member %s in pane %s: %w", member.PaneTitle, paneID, err)
			}
			s.deps.SleepFn(cdDelay)
		}

		launchCommand := buildLaunchCommand(member.Command, member.Args)
		slog.Debug("[DEBUG-SENDKEYS] bootstrap-member launch command", "paneID", paneID, "member", member.PaneTitle, "fullText", launchCommand)
		if err := s.deps.SendKeys(paneID, launchCommand); err != nil {
			return result, fmt.Errorf("failed to launch member %s in pane %s: %w", member.PaneTitle, paneID, err)
		}

		s.deps.SleepFn(time.Duration(request.BootstrapDelayMs) * time.Millisecond)
	}

	agentNames := DeriveAgentNames([]TeamMember{member})
	agentName := agentNames[member.ID]

	bootstrapMessage := BuildBootstrapMessage(request.TeamName, member, paneID, agentName)
	slog.Debug("[DEBUG-SENDKEYS] bootstrap-member message", "paneID", paneID, "member", member.PaneTitle, "fullText", bootstrapMessage)

	if IsClaudeCommand(member.Command) {
		if err := s.deps.SendKeysPaste(paneID, bootstrapMessage); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", member.PaneTitle, paneID, err))
		}
	} else {
		if err := s.deps.SendKeys(paneID, bootstrapMessage); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", member.PaneTitle, paneID, err))
		}
	}

	return result, nil
}
