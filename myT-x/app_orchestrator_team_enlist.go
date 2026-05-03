package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	agentdomain "myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/orchestrator"
	"myT-x/internal/tmux"
)

// GetSessionEnlistmentContext returns the data required by the pane enlistment UI.
func (a *App) GetSessionEnlistmentContext(sessionName string) (OrchestratorSessionEnlistmentContext, error) {
	context, err := a.orchestratorService.GetSessionEnlistmentContext(sessionName)
	if err != nil {
		return OrchestratorSessionEnlistmentContext{}, err
	}
	registeredPaneIDs, err := a.listOrchestratorRegisteredPaneIDs(sessionName)
	if err != nil {
		return OrchestratorSessionEnlistmentContext{}, err
	}
	context.RegisteredPaneIDs = registeredPaneIDs
	return context, nil
}

// EnlistPane saves an existing pane as a team member and bootstraps it.
func (a *App) EnlistPane(request EnlistPaneRequest) (EnlistPaneResult, error) {
	request.Normalize()
	if err := request.Validate(); err != nil {
		return EnlistPaneResult{}, err
	}

	sessionSnapshot, err := a.sessionService.FindSessionSnapshotByName(request.SessionName)
	if err != nil {
		return EnlistPaneResult{}, err
	}
	if !sessionContainsPane(sessionSnapshot, request.PaneID) {
		return EnlistPaneResult{}, fmt.Errorf("pane %s not found in session %s", request.PaneID, request.SessionName)
	}
	existingAgentName, err := a.findOrchestratorAgentNameByPaneID(request.SessionName, request.PaneID)
	if err != nil {
		return EnlistPaneResult{}, err
	}
	if existingAgentName != "" {
		return EnlistPaneResult{}, fmt.Errorf("pane %s is already registered as agent %s", request.PaneID, existingAgentName)
	}

	context, err := a.orchestratorService.GetSessionEnlistmentContext(request.SessionName)
	if err != nil {
		return EnlistPaneResult{}, err
	}
	targetTeam, err := findEnlistmentTeam(context.Teams, request.TeamID, request.StorageLocation)
	if err != nil {
		return EnlistPaneResult{}, err
	}

	member := request.Member
	member.TeamID = targetTeam.ID
	agentName := orchestrator.DeriveAgentNames([]orchestrator.TeamMember{member})[member.ID]

	if orchestrator.IsSystemTeam(targetTeam.ID) {
		if err := a.orchestratorService.AddMemberToUnaffiliatedTeam(member, enlistmentStorageLocation(targetTeam), request.SessionName); err != nil {
			return EnlistPaneResult{}, err
		}
	} else {
		updatedTeam := cloneTeamDefinition(targetTeam)
		updatedTeam.StorageLocation = enlistmentStorageLocation(targetTeam)
		updatedTeam.Members = append(updatedTeam.Members, member)
		if err := a.orchestratorService.SaveTeam(updatedTeam, request.SessionName); err != nil {
			return EnlistPaneResult{}, err
		}
	}

	if err := a.provisionalRegisterOrchestratorAgent(request.SessionName, request.PaneID, agentName, member); err != nil {
		rollbackErr := a.rollbackEnlistedMember(request.SessionName, targetTeam, request.PaneID, agentName)
		return EnlistPaneResult{}, joinEnlistmentFailure("provisional registration failed", err, rollbackErr)
	}

	bootstrapResult, bootstrapErr := a.orchestratorService.BootstrapMemberToPane(orchestrator.BootstrapMemberToPaneRequest{
		PaneID:           request.PaneID,
		PaneState:        request.PaneState,
		TeamName:         targetTeam.Name,
		Member:           member,
		BootstrapDelayMs: request.BootstrapDelayMs,
		SessionName:      request.SessionName,
	})
	if bootstrapErr != nil || len(bootstrapResult.Warnings) > 0 {
		bootstrapFailure := bootstrapErr
		if bootstrapFailure == nil {
			bootstrapFailure = errors.New(strings.Join(bootstrapResult.Warnings, "; "))
		}
		rollbackErr := a.rollbackEnlistedMember(request.SessionName, targetTeam, request.PaneID, agentName)
		return EnlistPaneResult{}, joinEnlistmentFailure("bootstrap failed", bootstrapFailure, rollbackErr)
	}

	a.emitBackendEvent("orchestrator:agents-updated", map[string]any{"sessionName": request.SessionName})
	return EnlistPaneResult{Warnings: []string{}}, nil
}

func findEnlistmentTeam(teams []orchestrator.TeamDefinition, teamID string, storageLocation string) (orchestrator.TeamDefinition, error) {
	normalizedStorage := strings.TrimSpace(storageLocation)
	if normalizedStorage == "" {
		normalizedStorage = orchestrator.StorageLocationGlobal
	}

	for _, team := range teams {
		if team.ID != teamID {
			continue
		}
		teamStorage := enlistmentStorageLocation(team)
		if orchestrator.IsSystemTeam(team.ID) {
			if teamStorage == normalizedStorage {
				return team, nil
			}
			continue
		}
		if teamStorage == normalizedStorage {
			return team, nil
		}
	}
	return orchestrator.TeamDefinition{}, fmt.Errorf("team %q (%s) not found", teamID, normalizedStorage)
}

func enlistmentStorageLocation(team orchestrator.TeamDefinition) string {
	storageLocation := strings.TrimSpace(team.StorageLocation)
	if storageLocation == "" {
		return orchestrator.StorageLocationGlobal
	}
	return storageLocation
}

func cloneTeamDefinition(team orchestrator.TeamDefinition) orchestrator.TeamDefinition {
	cloned := team
	cloned.Members = append([]orchestrator.TeamMember{}, team.Members...)
	return cloned
}

func sessionContainsPane(snapshot tmux.SessionSnapshot, paneID string) bool {
	for _, window := range snapshot.Windows {
		for _, pane := range window.Panes {
			if pane.ID == paneID {
				return true
			}
		}
	}
	return false
}

func joinEnlistmentFailure(prefix string, cause error, rollbackErr error) error {
	if rollbackErr == nil {
		return fmt.Errorf("%s: %w", prefix, cause)
	}
	return fmt.Errorf("%s: %v (rollback failed: %w)", prefix, cause, rollbackErr)
}

func (a *App) rollbackEnlistedMember(sessionName string, originalTeam orchestrator.TeamDefinition, paneID string, agentName string) error {
	var rollbackErr error
	if orchestrator.IsSystemTeam(originalTeam.ID) {
		rollbackErr = a.orchestratorService.SaveUnaffiliatedTeamMembers(originalTeam.Members, sessionName)
	} else {
		rollbackErr = a.orchestratorService.SaveTeam(originalTeam, sessionName)
	}
	deleteErr := a.deleteProvisionalOrchestratorAgent(sessionName, paneID, agentName)
	return errors.Join(rollbackErr, deleteErr)
}

func (a *App) listOrchestratorRegisteredPaneIDs(sessionName string) ([]string, error) {
	db, cleanup, ok, err := a.openOrchestratorDBOptional(sessionName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []string{}, nil
	}
	defer cleanup()

	rows, err := db.Query(`SELECT DISTINCT pane_id FROM agents WHERE COALESCE(pane_id, '') <> '' ORDER BY pane_id`)
	if err != nil {
		return nil, fmt.Errorf("list registered pane ids: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// Close failures after iteration do not change the already collected pane IDs.
			// Log the issue so resource cleanup problems stay visible.
			slog.Warn("[WARN-orchestrator] failed to close registered pane id rows", "error", closeErr)
		}
	}()

	result := make([]string, 0)
	for rows.Next() {
		var paneID string
		if err := rows.Scan(&paneID); err != nil {
			return nil, fmt.Errorf("scan registered pane id: %w", err)
		}
		result = append(result, paneID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate registered pane ids: %w", err)
	}
	return result, nil
}

func (a *App) findOrchestratorAgentNameByPaneID(sessionName string, paneID string) (string, error) {
	db, cleanup, ok, err := a.openOrchestratorDBOptional(sessionName)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	defer cleanup()

	var agentName string
	row := db.QueryRow(`SELECT name FROM agents WHERE pane_id = ? LIMIT 1`, paneID)
	switch err := row.Scan(&agentName); {
	case err == nil:
		return agentName, nil
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	default:
		return "", fmt.Errorf("lookup agent by pane id %s: %w", paneID, err)
	}
}

func (a *App) deleteProvisionalOrchestratorAgent(sessionName, paneID, agentName string) error {
	db, cleanup, err := a.openOrchestratorDBWritable(sessionName)
	if err != nil {
		return err
	}
	defer cleanup()

	if _, err := db.Exec(`DELETE FROM agent_status WHERE agent_name = ?`, agentName); err != nil && !isSQLiteMissingTable(err, "agent_status") {
		return fmt.Errorf("delete provisional agent status for %s: %w", agentName, err)
	}
	if _, err := db.Exec(`DELETE FROM agents WHERE pane_id = ?`, paneID); err != nil {
		return fmt.Errorf("delete provisional agent for pane %s: %w", paneID, err)
	}
	return nil
}

func isSQLiteMissingTable(err error, tableName string) bool {
	return err != nil && strings.Contains(err.Error(), "no such table: "+tableName)
}

func (a *App) provisionalRegisterOrchestratorAgent(sessionName, paneID, agentName string, member orchestrator.TeamMember) error {
	db, cleanup, err := a.openOrchestratorDBWritable(sessionName)
	if err != nil {
		return err
	}
	defer cleanup()

	skillsJSON, err := json.Marshal(member.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin provisional agent registration: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := ensureProvisionalAgentNameAvailable(tx, agentName, paneID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM agents WHERE pane_id = ? AND name <> ?`, paneID, agentName); err != nil {
		return fmt.Errorf("delete existing pane registrations: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO agents (name, pane_id, role, skills, mcp_instance_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
			pane_id=excluded.pane_id,
			role=excluded.role,
			skills=excluded.skills,
			mcp_instance_id=COALESCE(agents.mcp_instance_id, excluded.mcp_instance_id)`,
		agentName,
		paneID,
		member.Role,
		string(skillsJSON),
		nil,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("upsert provisional agent: %w", err)
	}
	if _, err := tx.Exec(
		`CREATE TABLE IF NOT EXISTS agent_status (
			agent_name TEXT PRIMARY KEY REFERENCES agents(name) ON DELETE CASCADE,
			status TEXT NOT NULL,
			current_task_id TEXT,
			note TEXT,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	); err != nil {
		return fmt.Errorf("ensure provisional agent status table: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO agent_status (agent_name, status, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(agent_name) DO NOTHING`,
		agentName,
		string(agentdomain.AgentWorkStatusIdle),
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("upsert provisional agent status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provisional agent registration: %w", err)
	}
	return nil
}

func ensureProvisionalAgentNameAvailable(tx *sql.Tx, agentName, paneID string) error {
	var existingPaneID string
	row := tx.QueryRow(`SELECT pane_id FROM agents WHERE name = ?`, agentName)
	switch err := row.Scan(&existingPaneID); {
	case err == nil:
		if existingPaneID != paneID {
			return fmt.Errorf("agent name %q is already registered to pane %s", agentName, existingPaneID)
		}
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return nil
	default:
		return fmt.Errorf("lookup provisional agent conflict for %q: %w", agentName, err)
	}
}
