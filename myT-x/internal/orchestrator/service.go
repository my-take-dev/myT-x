package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	sample_teams "myT-x/embed/sample_teams"
	"myT-x/internal/config"
	"myT-x/internal/tmux"
)

// Deps holds external dependencies injected at construction time.
// All function fields except SleepFn must be non-nil.
// NewService panics if any required function field is nil.
//
// Optional:
//   - SleepFn: defaults to time.Sleep if nil.
type Deps struct {
	// ConfigPath returns the application config file path.
	// Used to resolve the global team storage directory.
	ConfigPath func() string

	// FindSessionSnapshot looks up a session by name.
	FindSessionSnapshot func(sessionName string) (tmux.SessionSnapshot, error)

	// GetActiveSessionName returns the current active session name.
	GetActiveSessionName func() string

	// CreateSession creates a new tmux session rooted at the given path.
	CreateSession func(rootPath, sessionName string) (tmux.SessionSnapshot, error)

	// CreatePaneInSession recreates the first pane in an existing empty session.
	CreatePaneInSession func(sessionName string) (string, error)

	// KillSession destroys a session (without deleting worktrees).
	KillSession func(sessionName string) error

	// SplitPane creates a new pane by splitting an existing one.
	SplitPane func(paneID string, horizontal bool) (string, error)

	// RenamePane renames a pane's title.
	RenamePane func(paneID, title string) error

	// ApplyLayoutPreset applies a named layout preset to a session.
	ApplyLayoutPreset func(sessionName, preset string) error

	// SendKeys sends keystrokes to a pane with Enter.
	SendKeys func(paneID, text string) error

	// SendKeysPaste sends keystrokes with bracketed paste mode and Enter.
	SendKeysPaste func(paneID, text string) error

	// SleepFn provides the sleep function (replaceable for testing).
	// Optional: defaults to time.Sleep if nil.
	SleepFn func(time.Duration)

	// CheckReady validates that the runtime environment (e.g. router) is ready.
	// Called at StartTeam entry to fail fast before any side effects.
	CheckReady func() error
}

// Service manages orchestrator team persistence and team launch operations.
//
// Thread-safety: mu protects team file I/O (CRUD operations).
// StartTeam calls deps functions (CreateSession, SplitPane, SendKeys, etc.)
// outside mu; those operations rely on the thread-safety of the injected deps.
// No external locking is required by callers.
type Service struct {
	deps Deps
	mu   sync.Mutex
	// seedOnce is per-Service instance (not package-level) so that each test
	// creates an independent Service with its own seeding state.
	seedOnce sync.Once
}

// NewService creates an orchestrator service with the given dependencies.
// Panics if any required function field in deps is nil.
func NewService(deps Deps) *Service {
	if deps.ConfigPath == nil || deps.FindSessionSnapshot == nil ||
		deps.GetActiveSessionName == nil || deps.CreateSession == nil ||
		deps.CreatePaneInSession == nil ||
		deps.KillSession == nil || deps.SplitPane == nil ||
		deps.RenamePane == nil || deps.ApplyLayoutPreset == nil ||
		deps.SendKeys == nil || deps.SendKeysPaste == nil ||
		deps.CheckReady == nil {
		panic("orchestrator.NewService: required function fields in Deps must be non-nil " +
			"(ConfigPath, FindSessionSnapshot, GetActiveSessionName, CreateSession, " +
			"CreatePaneInSession, KillSession, SplitPane, RenamePane, ApplyLayoutPreset, " +
			"SendKeys, SendKeysPaste, CheckReady)")
	}
	if deps.SleepFn == nil {
		deps.SleepFn = time.Sleep
	}
	return &Service{deps: deps}
}

// ------------------------------------------------------------
// CRUD operations
// ------------------------------------------------------------

// SaveTeam saves or updates a team definition.
// System teams cannot be saved through this method; use the dedicated
// AddMemberToUnaffiliatedTeam endpoint for member management instead.
func (s *Service) SaveTeam(team TeamDefinition, sessionName string) error {
	if IsSystemTeam(strings.TrimSpace(team.ID)) {
		return fmt.Errorf("cannot overwrite system team %q via SaveTeam", team.ID)
	}

	storageLocation := strings.TrimSpace(team.StorageLocation)
	team.StorageLocation = "" // Do not persist storage_location to file.
	team.Normalize()
	if err := team.Validate(); err != nil {
		return err
	}

	var definitionsPath, membersPath string
	if storageLocation == StorageLocationProject {
		var err error
		definitionsPath, membersPath, err = s.resolveProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = s.resolveGlobalStoragePaths()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	members, err := readMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	for _, d := range definitions {
		if d.ID != team.ID && strings.TrimSpace(d.Name) == strings.TrimSpace(team.Name) {
			return fmt.Errorf("team name %q already exists", team.Name)
		}
	}

	definitions = upsertDefinition(definitions, team)
	members = upsertMembers(members, team)
	members = filterOrphanMembers(members, definitions)
	sortDefinitions(definitions)
	sortMembers(members)

	// Two-file write strategy:
	// - If definitions write succeeds but members write fails: orphan members from
	//   a previous state may be stale, but filterOrphanMembers cleans
	//   them up on next load. The new definition exists without its members.
	// - If definitions write fails: we return early without updating members,
	//   preserving the previous consistent state.
	if err := writeDefinitions(definitionsPath, definitions); err != nil {
		return err
	}
	if err := writeMembers(membersPath, members); err != nil {
		return err
	}
	return nil
}

// LoadTeams loads global and project-local team definitions.
func (s *Service) LoadTeams(sessionName string) ([]TeamDefinition, error) {
	definitionsPath, membersPath := s.resolveGlobalStoragePaths()

	// Seed sample teams outside the lock (file I/O may be slow).
	// seedSamples is idempotent (checks file existence).
	s.seedSamples(definitionsPath, membersPath)

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitions(definitionsPath)
	if err != nil {
		return []TeamDefinition{}, fmt.Errorf("read team definitions: %w", err)
	}

	members, err := readMembers(membersPath)
	if err != nil {
		return []TeamDefinition{}, fmt.Errorf("read team members: %w", err)
	}
	globalTeams := joinTeams(definitions, members)
	for i := range globalTeams {
		globalTeams[i].StorageLocation = StorageLocationGlobal
	}

	// Merge project-local teams if session is specified.
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return globalTeams, nil
	}
	projDefPath, projMemPath, err := s.resolveProjectStoragePaths(sessionName)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project storage path resolution skipped", "error", err)
		return globalTeams, nil
	}
	projDefs, err := readDefinitions(projDefPath)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project team definitions not available", "error", err)
		return globalTeams, nil
	}
	projMembers, err := readMembers(projMemPath)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] project team members not available", "error", err)
		projMembers = []TeamMember{}
	}
	projectTeams := joinTeams(projDefs, projMembers)
	for i := range projectTeams {
		projectTeams[i].StorageLocation = StorageLocationProject
	}

	return append(globalTeams, projectTeams...), nil
}

// DeleteTeam deletes a team definition.
func (s *Service) DeleteTeam(teamID string, storageLocation string, sessionName string) error {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return errors.New("team id is required")
	}
	if IsSystemTeam(teamID) {
		return fmt.Errorf("cannot delete system team %q", teamID)
	}

	var definitionsPath, membersPath string
	if strings.TrimSpace(storageLocation) == StorageLocationProject {
		var err error
		definitionsPath, membersPath, err = s.resolveProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = s.resolveGlobalStoragePaths()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	members, err := readMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	filteredDefinitions := make([]teamFileRecord, 0, len(definitions))
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

	filteredMembers := make([]TeamMember, 0, len(members))
	for _, member := range members {
		if member.TeamID == teamID {
			continue
		}
		filteredMembers = append(filteredMembers, member)
	}
	filteredMembers = filterOrphanMembers(filteredMembers, filteredDefinitions)
	sortDefinitions(filteredDefinitions)
	sortMembers(filteredMembers)

	// Two-file write strategy:
	// - If definitions write succeeds but members write fails: the deleted team's
	//   members remain but are cleaned up by filterOrphanMembers on next load.
	// - If definitions write fails: we return early without updating members,
	//   preserving the previous consistent state.
	if err := writeDefinitions(definitionsPath, filteredDefinitions); err != nil {
		return err
	}
	if err := writeMembers(membersPath, filteredMembers); err != nil {
		return err
	}
	return nil
}

// EnsureUnaffiliatedTeam returns the unaffiliated (system) team for the given
// storage location, creating it if it does not yet exist.
func (s *Service) EnsureUnaffiliatedTeam(storageLocation string, sessionName string) (TeamDefinition, error) {
	storageLocation = strings.TrimSpace(storageLocation)
	if storageLocation == "" {
		storageLocation = StorageLocationGlobal
	}

	var definitionsPath, membersPath string
	if storageLocation == StorageLocationProject {
		var err error
		definitionsPath, membersPath, err = s.resolveProjectStoragePaths(sessionName)
		if err != nil {
			return TeamDefinition{}, fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = s.resolveGlobalStoragePaths()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return TeamDefinition{}, fmt.Errorf("read team definitions: %w", err)
	}
	members, err := readMembersForWrite(membersPath)
	if err != nil {
		return TeamDefinition{}, fmt.Errorf("read team members: %w", err)
	}

	// Check if unaffiliated team already exists.
	for _, d := range definitions {
		if IsSystemTeam(d.ID) {
			team := TeamDefinition{
				ID:               d.ID,
				Name:             d.Name,
				Description:      d.Description,
				Order:            d.Order,
				BootstrapDelayMs: d.BootstrapDelayMs,
				StorageLocation:  storageLocation,
			}
			for _, m := range members {
				if IsSystemTeam(m.TeamID) {
					team.Members = append(team.Members, m)
				}
			}
			if team.Members == nil {
				team.Members = []TeamMember{}
			}
			return team, nil
		}
	}

	// Create the unaffiliated team (definitions-only write).
	// Members are not written here because the team starts empty.
	// AddMemberToUnaffiliatedTeam handles the members write separately,
	// following the same Two-file write strategy as SaveTeam.
	newDef := newUnaffiliatedTeamRecord()
	definitions = append(definitions, newDef)
	sortDefinitions(definitions)

	if err := writeDefinitions(definitionsPath, definitions); err != nil {
		return TeamDefinition{}, fmt.Errorf("write definitions: %w", err)
	}

	return TeamDefinition{
		ID:               newDef.ID,
		Name:             newDef.Name,
		Description:      newDef.Description,
		Order:            newDef.Order,
		BootstrapDelayMs: newDef.BootstrapDelayMs,
		StorageLocation:  storageLocation,
		Members:          []TeamMember{},
	}, nil
}

// AddMemberToUnaffiliatedTeam adds a member to the unaffiliated team,
// creating the team if it does not yet exist.
func (s *Service) AddMemberToUnaffiliatedTeam(member TeamMember, storageLocation string, sessionName string) error {
	storageLocation = strings.TrimSpace(storageLocation)
	if storageLocation == "" {
		storageLocation = StorageLocationGlobal
	}

	member.Normalize()
	member.TeamID = UnaffiliatedTeamID
	if err := member.Validate(); err != nil {
		return fmt.Errorf("member validation failed: %w", err)
	}

	var definitionsPath, membersPath string
	if storageLocation == StorageLocationProject {
		var err error
		definitionsPath, membersPath, err = s.resolveProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, membersPath = s.resolveGlobalStoragePaths()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	allMembers, err := readMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	// Ensure unaffiliated team definition exists.
	found := false
	for _, d := range definitions {
		if IsSystemTeam(d.ID) {
			found = true
			break
		}
	}
	if !found {
		definitions = append(definitions, newUnaffiliatedTeamRecord())
		sortDefinitions(definitions)
	}

	// Reject duplicate pane_title within the unaffiliated team.
	newTitle := strings.TrimSpace(member.PaneTitle)
	for _, m := range allMembers {
		if IsSystemTeam(m.TeamID) && strings.TrimSpace(m.PaneTitle) == newTitle {
			return fmt.Errorf("duplicate pane title %q in unaffiliated team", newTitle)
		}
	}

	// Set order for the new member.
	maxOrder := -1
	for _, m := range allMembers {
		if IsSystemTeam(m.TeamID) && m.Order > maxOrder {
			maxOrder = m.Order
		}
	}
	member.Order = maxOrder + 1

	allMembers = append(allMembers, member)
	sortMembers(allMembers)

	// Two-file write strategy (same as SaveTeam):
	// Write definitions only when a new team definition was created.
	// If definitions write succeeds but members write fails, the team definition
	// exists without its new member — consistent with the existing partial-failure model.
	if !found {
		if err := writeDefinitions(definitionsPath, definitions); err != nil {
			return fmt.Errorf("write definitions: %w", err)
		}
	}
	if err := writeMembers(membersPath, allMembers); err != nil {
		return fmt.Errorf("write members: %w", err)
	}
	return nil
}

// SaveUnaffiliatedTeamMembers replaces all members in the unaffiliated team
// with the given members slice. An empty slice is valid and removes all members.
// Storage is always global, so sessionName is accepted only for API symmetry.
func (s *Service) SaveUnaffiliatedTeamMembers(members []TeamMember, sessionName string) error {
	_ = sessionName

	normalizedMembers := make([]TeamMember, len(members))
	copy(normalizedMembers, members)

	// Normalize and validate each member without mutating the caller's slice.
	seen := make(map[string]struct{}, len(normalizedMembers))
	for i := range normalizedMembers {
		normalizedMembers[i].Normalize()
		normalizedMembers[i].TeamID = UnaffiliatedTeamID
		normalizedMembers[i].Order = i
		if err := normalizedMembers[i].Validate(); err != nil {
			return fmt.Errorf("member[%d] validation failed: %w", i, err)
		}
		title := strings.TrimSpace(normalizedMembers[i].PaneTitle)
		if _, dup := seen[title]; dup {
			return fmt.Errorf("duplicate pane title %q in input members", title)
		}
		seen[title] = struct{}{}
	}

	definitionsPath, membersPath := s.resolveGlobalStoragePaths()

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}
	allMembers, err := readMembersForWrite(membersPath)
	if err != nil {
		return fmt.Errorf("read team members: %w", err)
	}

	// Ensure unaffiliated team definition exists.
	defFound := false
	for _, d := range definitions {
		if IsSystemTeam(d.ID) {
			defFound = true
			break
		}
	}
	if !defFound {
		definitions = append(definitions, newUnaffiliatedTeamRecord())
		sortDefinitions(definitions)
	}

	// Remove existing unaffiliated members, keep others intact.
	kept := make([]TeamMember, 0, len(allMembers))
	for _, m := range allMembers {
		if !IsSystemTeam(m.TeamID) {
			kept = append(kept, m)
		}
	}
	// Append the new members.
	kept = append(kept, normalizedMembers...)
	kept = filterOrphanMembers(kept, definitions)
	sortMembers(kept)

	// Two-file write strategy: write definitions only when newly created.
	if !defFound {
		if err := writeDefinitions(definitionsPath, definitions); err != nil {
			return fmt.Errorf("write definitions: %w", err)
		}
	}
	if err := writeMembers(membersPath, kept); err != nil {
		return fmt.Errorf("write members: %w", err)
	}
	return nil
}

// ReorderTeams reorders team definitions by the given ID sequence.
func (s *Service) ReorderTeams(teamIDs []string, storageLocation string, sessionName string) error {
	if len(teamIDs) == 0 {
		return nil
	}

	var definitionsPath string
	if strings.TrimSpace(storageLocation) == StorageLocationProject {
		var err error
		definitionsPath, _, err = s.resolveProjectStoragePaths(sessionName)
		if err != nil {
			return fmt.Errorf("resolve project storage: %w", err)
		}
	} else {
		definitionsPath, _ = s.resolveGlobalStoragePaths()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	definitions, err := readDefinitionsForWrite(definitionsPath)
	if err != nil {
		return fmt.Errorf("read team definitions: %w", err)
	}

	byID := make(map[string]*teamFileRecord, len(definitions))
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

	sortDefinitions(definitions)

	if err := writeDefinitions(definitionsPath, definitions); err != nil {
		return err
	}
	return nil
}

// ResolveGlobalStoragePaths returns the global storage paths for definitions and members.
func (s *Service) ResolveGlobalStoragePaths() (string, string) {
	return s.resolveGlobalStoragePaths()
}

// newUnaffiliatedTeamRecord creates the on-disk representation for the
// unaffiliated (system) team with default values.
func newUnaffiliatedTeamRecord() teamFileRecord {
	return teamFileRecord{
		ID:               UnaffiliatedTeamID,
		Name:             UnaffiliatedTeamName,
		Description:      UnaffiliatedTeamDescription,
		Order:            UnaffiliatedTeamOrder,
		BootstrapDelayMs: BootstrapDelayMsDefault,
	}
}

// ------------------------------------------------------------
// Team startup
// ------------------------------------------------------------

// launchedMember tracks a member that completed Phase 1 (command execution)
// and is ready for Phase 2 (bootstrap message injection).
type launchedMember struct {
	member TeamMember
	paneID string
}

// StartTeam launches a team of agents using a two-phase approach:
//
//	Phase 1 — Execute commands (fast): RenamePane → cd → launch for each member.
//	Phase 2 — Inject roles (one wait): sleep(BootstrapDelayMs) once → send bootstrap messages sequentially.
//
// This reduces startup time from O(N × BootstrapDelayMs) to O(BootstrapDelayMs).
func (s *Service) StartTeam(request StartTeamRequest) (StartTeamResult, error) {
	// Fail fast before any side effects (session creation, sleep, etc.).
	if err := s.deps.CheckReady(); err != nil {
		return StartTeamResult{}, fmt.Errorf("runtime not ready: %w", err)
	}

	request.Normalize()
	if err := request.Validate(); err != nil {
		return StartTeamResult{}, err
	}

	sourceSessionName := request.SourceSessionName
	if sourceSessionName == "" {
		sourceSessionName = s.deps.GetActiveSessionName()
	}

	teams, err := s.LoadTeams(sourceSessionName)
	if err != nil {
		return StartTeamResult{}, err
	}
	team, err := findTeamByID(teams, request.TeamID)
	if err != nil {
		return StartTeamResult{}, err
	}
	if IsSystemTeam(team.ID) {
		return StartTeamResult{}, fmt.Errorf("system team %q cannot be started directly", team.Name)
	}
	if len(team.Members) == 0 {
		return StartTeamResult{}, errors.New("team has no members")
	}

	sourceSession, err := s.deps.FindSessionSnapshot(sourceSessionName)
	if err != nil {
		return StartTeamResult{}, err
	}
	sourceRootPath, err := ResolveSourceRootPath(sourceSession)
	if err != nil {
		return StartTeamResult{}, err
	}

	sessionName, panes, createdNewSession, warnings, err := s.prepareLaunchTarget(team, request, sourceRootPath, sourceSession)
	if err != nil {
		if createdNewSession && strings.TrimSpace(sessionName) != "" {
			if rollbackErr := s.deps.KillSession(sessionName); rollbackErr != nil {
				slog.Warn("[WARN-ORCH-TEAM] failed to rollback new session after target preparation failure",
					"session", sessionName, "error", rollbackErr)
			}
		}
		return StartTeamResult{}, err
	}

	result := StartTeamResult{
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
			if err := s.deps.KillSession(sessionName); err != nil {
				slog.Warn("[WARN-ORCH-TEAM] failed to rollback new session after pre-launch failure",
					"session", sessionName, "error", err)
			}
		}()
	}

	// Wait for shells in newly created panes to initialize.
	s.deps.SleepFn(shellInitDelay)

	agentNames := DeriveAgentNames(team.Members)

	// ── Phase 1: Execute commands (fast) ──
	// Launch all CLIs in rapid succession without waiting for each to start.
	launched := make([]launchedMember, 0, len(team.Members))
	for index, member := range team.Members {
		if index >= len(panes) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("skipped member %s: no pane available", member.PaneTitle))
			continue
		}

		paneID := panes[index].ID
		result.MemberPaneIDs[member.ID] = paneID

		if err := s.deps.RenamePane(paneID, member.PaneTitle); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to rename pane %s for member %s: %v", paneID, member.PaneTitle, err))
		}

		if strings.TrimSpace(sourceRootPath) != "" {
			// Use double quotes for Windows PowerShell compatibility.
			cdCommand := fmt.Sprintf(`cd "%s"`, strings.ReplaceAll(sourceRootPath, `"`, `\"`))
			slog.Info("[DEBUG-SENDKEYS] cd command", "paneID", paneID, "member", member.PaneTitle, "fullText", cdCommand)
			if err := s.deps.SendKeys(paneID, cdCommand); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("cd failed for member %s in pane %s (skipping launch): %v", member.PaneTitle, paneID, err))
				continue
			}
			s.deps.SleepFn(cdDelay)
		}

		launchCommand := buildLaunchCommand(member.Command, member.Args)
		slog.Info("[DEBUG-SENDKEYS] launch command", "paneID", paneID, "member", member.PaneTitle, "fullText", launchCommand)
		if err := s.deps.SendKeys(paneID, launchCommand); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to launch member %s in pane %s: %v", member.PaneTitle, paneID, err))
			continue
		}
		injectedAnyCommand = true
		launched = append(launched, launchedMember{member: member, paneID: paneID})
	}

	// ── Phase 2: Inject bootstrap messages (one wait) ──
	// Wait once for all CLIs to start, then send bootstrap messages sequentially.
	if len(launched) > 0 {
		s.deps.SleepFn(time.Duration(team.BootstrapDelayMs) * time.Millisecond)

		for i, lm := range launched {
			if i > 0 {
				s.deps.SleepFn(bootstrapInterMessageDelay)
			}

			bootstrapMessage := BuildBootstrapMessage(team.Name, lm.member, lm.paneID, agentNames[lm.member.ID])
			slog.Info("[DEBUG-SENDKEYS] bootstrap message", "paneID", lm.paneID, "member", lm.member.PaneTitle, "fullText", bootstrapMessage)
			// Claude Code treats \n as Enter/submit in its terminal UI.
			// Use bracketed paste mode so the entire message is received as one input.
			if IsClaudeCommand(lm.member.Command) {
				if err := s.deps.SendKeysPaste(lm.paneID, bootstrapMessage); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", lm.member.PaneTitle, lm.paneID, err))
				}
			} else {
				if err := s.deps.SendKeys(lm.paneID, bootstrapMessage); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to send bootstrap to member %s in pane %s: %v", lm.member.PaneTitle, lm.paneID, err))
				}
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

// ------------------------------------------------------------
// Internal: path resolution
// ------------------------------------------------------------

func (s *Service) resolveGlobalStoragePaths() (string, string) {
	configPath := strings.TrimSpace(s.deps.ConfigPath())
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	baseDir := filepath.Dir(configPath)
	return filepath.Join(baseDir, definitionsFileName), filepath.Join(baseDir, membersFileName)
}

func (s *Service) resolveProjectStoragePaths(sessionName string) (string, string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", "", errors.New("session name is required for project storage")
	}
	snapshot, err := s.deps.FindSessionSnapshot(sessionName)
	if err != nil {
		return "", "", err
	}
	rootPath, err := ResolveSourceRootPath(snapshot)
	if err != nil {
		return "", "", err
	}
	baseDir := filepath.Join(rootPath, ".myT-x")
	return filepath.Join(baseDir, definitionsFileName),
		filepath.Join(baseDir, membersFileName), nil
}

// ------------------------------------------------------------
// Internal: launch target preparation
// ------------------------------------------------------------

func (s *Service) prepareLaunchTarget(
	team TeamDefinition,
	request StartTeamRequest,
	sourceRootPath string,
	sourceSession tmux.SessionSnapshot,
) (string, []tmux.PaneSnapshot, bool, []string, error) {
	switch request.LaunchMode {
	case LaunchModeActiveSession:
		activeWindow := resolveActiveWindow(sourceSession.Windows, sourceSession.ActiveWindowID)
		panes := []tmux.PaneSnapshot{}
		if activeWindow != nil {
			panes = cloneAndSortPanes(activeWindow.Panes)
		}
		// activeWindow==nil means the session is empty; ensurePaneCapacity will create
		// the initial pane via CreatePaneInSession before splitting for additional members.
		panes, warnings, err := s.ensurePaneCapacity(sourceSession.Name, panes, len(team.Members), true)
		if err != nil {
			return "", nil, false, nil, err
		}
		return sourceSession.Name, panes, false, warnings, nil
	case LaunchModeNewSession:
		sessionName := request.NewSessionName
		if sessionName == "" {
			sessionName = sanitizeSessionName(team.Name, "orchestrator-team")
		}
		createdSession, err := s.deps.CreateSession(sourceRootPath, sessionName)
		if err != nil {
			return "", nil, false, nil, err
		}
		activeWindow := resolveActiveWindow(createdSession.Windows, createdSession.ActiveWindowID)
		if activeWindow == nil {
			// Return createdSession.Name so the caller can rollback the session.
			return createdSession.Name, nil, true, nil, fmt.Errorf("session %s has no active window", createdSession.Name)
		}
		panes := cloneAndSortPanes(activeWindow.Panes)
		panes, warnings, err := s.ensurePaneCapacity(createdSession.Name, panes, len(team.Members), false)
		if err != nil {
			return createdSession.Name, nil, true, nil, err
		}
		if len(panes) > 1 {
			if err := s.deps.ApplyLayoutPreset(createdSession.Name, "tiled"); err != nil {
				return createdSession.Name, nil, true, nil, fmt.Errorf("apply tiled layout: %w", err)
			}
		}
		return createdSession.Name, panes, true, warnings, nil
	default:
		return "", nil, false, nil, fmt.Errorf("unsupported launch mode: %s", request.LaunchMode)
	}
}

func (s *Service) ensurePaneCapacity(
	sessionName string,
	panes []tmux.PaneSnapshot,
	requiredCount int,
	allowPartial bool,
) ([]tmux.PaneSnapshot, []string, error) {
	if requiredCount < 1 {
		return []tmux.PaneSnapshot{}, nil, nil
	}
	working := append([]tmux.PaneSnapshot{}, panes...)
	warnings := make([]string, 0)
	if len(working) == 0 {
		initialPaneID, err := s.deps.CreatePaneInSession(sessionName)
		if err != nil {
			return nil, nil, fmt.Errorf("create initial pane in session %s: %w", sessionName, err)
		}
		initialPaneID = strings.TrimSpace(initialPaneID)
		if initialPaneID == "" {
			return nil, nil, fmt.Errorf("create initial pane in session %s: empty pane id returned", sessionName)
		}
		working = append(working, tmux.PaneSnapshot{
			ID:    initialPaneID,
			Index: 0,
			// Only ID is used as the SplitPane source; other fields are intentionally zero.
		})
	}
	for len(working) < requiredCount {
		sourcePaneID := working[len(working)-1].ID
		newPaneID, err := s.deps.SplitPane(sourcePaneID, false)
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

// ------------------------------------------------------------
// Internal: CRUD helpers (file I/O)
// ------------------------------------------------------------

func upsertDefinition(definitions []teamFileRecord, team TeamDefinition) []teamFileRecord {
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
		definitions = append(definitions, teamFileRecord{
			ID:               team.ID,
			Name:             team.Name,
			Description:      team.Description,
			Order:            len(definitions),
			BootstrapDelayMs: team.BootstrapDelayMs,
		})
	}
	return definitions
}

func upsertMembers(existing []TeamMember, team TeamDefinition) []TeamMember {
	filtered := make([]TeamMember, 0, len(existing)+len(team.Members))
	for _, member := range existing {
		if member.TeamID == team.ID {
			continue
		}
		filtered = append(filtered, member)
	}
	filtered = append(filtered, team.Members...)
	return filtered
}

func filterOrphanMembers(members []TeamMember, definitions []teamFileRecord) []TeamMember {
	if len(members) == 0 {
		return []TeamMember{}
	}
	teamIDs := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		teamIDs[definition.ID] = struct{}{}
	}
	filtered := make([]TeamMember, 0, len(members))
	for _, member := range members {
		if _, exists := teamIDs[member.TeamID]; !exists {
			continue
		}
		filtered = append(filtered, member)
	}
	return filtered
}

func joinTeams(definitions []teamFileRecord, members []TeamMember) []TeamDefinition {
	joined := make([]TeamDefinition, 0, len(definitions))
	membersByTeam := make(map[string][]TeamMember, len(definitions))
	for _, member := range members {
		membersByTeam[member.TeamID] = append(membersByTeam[member.TeamID], member)
	}
	for _, definition := range definitions {
		teamMembers := append([]TeamMember{}, membersByTeam[definition.ID]...)
		sortMembers(teamMembers)
		delayMs := definition.BootstrapDelayMs
		if delayMs <= 0 {
			delayMs = BootstrapDelayMsDefault
		}
		joined = append(joined, TeamDefinition{
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

func sortDefinitions(definitions []teamFileRecord) {
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

func sortMembers(members []TeamMember) {
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

func readDefinitions(path string) ([]teamFileRecord, error) {
	return readDefinitionsWithMode(path, true)
}

func readDefinitionsForWrite(path string) ([]teamFileRecord, error) {
	return readDefinitionsWithMode(path, false)
}

func readDefinitionsWithMode(path string, allowMalformed bool) ([]teamFileRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []teamFileRecord{}, nil
		}
		return nil, err
	}

	var definitions []teamFileRecord
	if err := json.Unmarshal(data, &definitions); err != nil {
		// Create a .bak backup before potentially losing the malformed data.
		backupTeamFile(path, data)
		if allowMalformed {
			slog.Debug("[DEBUG-ORCH-TEAM] failed to parse team definitions, returning empty", "path", path, "error", err)
			return []teamFileRecord{}, nil
		}
		slog.Warn("[WARN-ORCH-TEAM] failed to parse team definitions, refusing to overwrite", "path", path, "error", err)
		return nil, fmt.Errorf("parse team definitions: %w", err)
	}
	for index := range definitions {
		definitions[index].ID = strings.TrimSpace(definitions[index].ID)
		definitions[index].Name = strings.TrimSpace(definitions[index].Name)
	}
	return definitions, nil
}

func readMembers(path string) ([]TeamMember, error) {
	return readMembersWithMode(path, true)
}

func readMembersForWrite(path string) ([]TeamMember, error) {
	return readMembersWithMode(path, false)
}

func readMembersWithMode(path string, allowMalformed bool) ([]TeamMember, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TeamMember{}, nil
		}
		return nil, err
	}

	var members []TeamMember
	if err := json.Unmarshal(data, &members); err != nil {
		backupTeamFile(path, data)
		if allowMalformed {
			slog.Debug("[DEBUG-ORCH-TEAM] failed to parse team members, returning empty", "path", path, "error", err)
			return []TeamMember{}, nil
		}
		slog.Warn("[WARN-ORCH-TEAM] failed to parse team members, refusing to overwrite", "path", path, "error", err)
		return nil, fmt.Errorf("parse team members: %w", err)
	}
	for index := range members {
		members[index].Normalize()
	}
	return members, nil
}

func writeDefinitions(path string, definitions []teamFileRecord) error {
	return writeTeamJSON(path, definitions, "team definitions")
}

func writeMembers(path string, members []TeamMember) error {
	return writeTeamJSON(path, members, "team members")
}

// backupTeamFile creates a .bak file to prevent data loss on parse failure.
// Best-effort: backup creation failure is logged but does not abort the caller.
func backupTeamFile(path string, data []byte) {
	bakPath := path + ".bak"
	if err := os.WriteFile(bakPath, data, 0o644); err != nil {
		slog.Warn("[WARN-ORCH-TEAM] failed to create backup of malformed file", "path", bakPath, "error", err)
	} else {
		slog.Info("[WARN-ORCH-TEAM] created backup of malformed file", "path", bakPath)
	}
}

func writeTeamJSON(path string, payload any, label string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create orchestrator team directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", label, err)
	}
	data = append(data, '\n') // trailing newline

	// Atomic write: temp file + rename
	tmpFile, err := os.CreateTemp(dir, ".team.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", label, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on error
		if tmpFile != nil {
			tmpFile.Close()
		}
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			if removeErr := os.Remove(tmpPath); removeErr != nil {
				slog.Debug("[DEBUG-ORCH-TEAM] failed to remove temp file", "path", tmpPath, "error", removeErr)
			}
		}
	}()
	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", label, err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", label, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", label, err)
	}
	tmpFile = nil
	if err := renameWithRetry(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", label, err)
	}
	return nil
}

func renameWithRetry(src, dst string) error {
	var lastErr error
	for attempt := range renameRetryMax {
		err := os.Rename(src, dst)
		if err == nil {
			if attempt > 0 {
				slog.Debug("[DEBUG-ORCH-TEAM] rename succeeded after retry", "src", src, "dst", dst, "attempt", attempt+1)
			}
			return nil
		}
		lastErr = err
		if runtime.GOOS != "windows" {
			return err
		}
		slog.Debug("[DEBUG-ORCH-TEAM] rename retry", "src", src, "dst", dst, "attempt", attempt+1, "error", err)
		time.Sleep(time.Duration(attempt+1) * renameRetryBaseDelay)
	}
	return lastErr
}

// seedSamples writes embedded sample teams if the definitions file does not exist.
// sync.Once ensures this runs at most once per Service instance.
func (s *Service) seedSamples(definitionsPath, membersPath string) {
	s.seedOnce.Do(func() {
		seedSamplesInternal(definitionsPath, membersPath)
	})
}

func seedSamplesInternal(definitionsPath, membersPath string) bool {
	if _, err := os.Stat(definitionsPath); err == nil {
		return false
	} else if !os.IsNotExist(err) {
		slog.Debug("[DEBUG-ORCH-TEAM] cannot check sample definitions file", "path", definitionsPath, "error", err)
		return false
	}

	defsData, err := sample_teams.FS.ReadFile(definitionsFileName)
	if err != nil {
		// Embedded file read failure indicates a build/packaging issue.
		slog.Warn("[WARN-ORCH-TEAM] failed to read embedded sample definitions", "error", err)
		return false
	}
	membersData, err := sample_teams.FS.ReadFile(membersFileName)
	if err != nil {
		slog.Warn("[WARN-ORCH-TEAM] failed to read embedded sample members", "error", err)
		return false
	}

	// Parse embedded data to use writeTeamJSON for atomic writes.
	var members []TeamMember
	if err := json.Unmarshal(membersData, &members); err != nil {
		slog.Warn("[WARN-ORCH-TEAM] failed to parse embedded sample members", "error", err)
		return false
	}
	var definitions []teamFileRecord
	if err := json.Unmarshal(defsData, &definitions); err != nil {
		slog.Warn("[WARN-ORCH-TEAM] failed to parse embedded sample definitions", "error", err)
		return false
	}

	// Write members first, then definitions. The definitions file is used as the
	// existence check for seeding, so writing it last ensures that if the process
	// is interrupted after members are written, the next startup will re-seed both.
	if err := writeMembers(membersPath, members); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to write sample members", "error", err)
		return false
	}
	if err := writeDefinitions(definitionsPath, definitions); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to write sample definitions", "error", err)
		return false
	}
	slog.Info("[DEBUG-ORCH-TEAM] seeded sample team definitions for first-time use")
	return true
}

// ------------------------------------------------------------
// Internal: start helpers
// ------------------------------------------------------------

// ResolveSourceRootPath returns the filesystem root path for the given session,
// preferring the worktree path if available.
func ResolveSourceRootPath(session tmux.SessionSnapshot) (string, error) {
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

// resolveActiveWindow delegates to the shared tmux utility.
func resolveActiveWindow(windows []tmux.WindowSnapshot, activeWindowID int) *tmux.WindowSnapshot {
	return tmux.ResolveActiveWindow(windows, activeWindowID)
}

// sanitizeSessionName delegates to the shared tmux utility.
func sanitizeSessionName(name, fallback string) string {
	return tmux.SanitizeSessionName(name, fallback)
}

// IsClaudeCommand returns true if the command refers to Claude Code CLI.
// Claude Code requires bracketed paste mode for multi-line input
// because it treats \n as Enter/submit in its terminal UI.
func IsClaudeCommand(command string) bool {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return false
	}
	base := strings.ToLower(filepath.Base(parts[0]))
	return base == "claude" || base == "claude.exe" || strings.HasPrefix(base, "claude-code")
}

func buildLaunchCommand(command string, args []string) string {
	parts := []string{strings.TrimSpace(command)}
	for _, arg := range args {
		parts = append(parts, QuoteCommandArg(arg))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// QuoteCommandArg quotes a command argument for Windows PowerShell compatibility.
func QuoteCommandArg(arg string) string {
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

// BuildBootstrapMessage generates the bootstrap instruction message for a team member.
func BuildBootstrapMessage(teamName string, member TeamMember, paneID, agentName string) string {
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

	if hints := BuildSkillCompletionHints(member.Role, member.Skills); hints != "" {
		builder.WriteString("\n--- 得意分野の補完 ---\n")
		builder.WriteString(hints)
	}

	builder.WriteString("\n--- ワークフロー ---\n")
	builder.WriteString("1. register_agent → 自身を登録（必須・最初に実行）\n")
	builder.WriteString("2. list_agents → チームメンバーとペイン状態を確認\n")
	builder.WriteString("3. send_task → 他エージェントにタスクを依頼（from_agent=自分の名前）\n")
	builder.WriteString("4. get_my_tasks → 自分宛タスクを確認（デフォルト: pending のみ）\n")
	builder.WriteString("5. send_response → タスクに返信し completed に更新（task_id 必須）\n")
	builder.WriteString("\nタスク状態: pending → completed / failed / abandoned\n")
	builder.WriteString("確認: list_all_tasks で全タスク一覧、capture_pane で相手の画面を取得\n")
	builder.WriteString("注意: send_task は応答テンプレートを自動付与（include_response_instructions=false で無効化可）\n")

	return builder.String()
}

// BuildSkillCompletionHints generates skill auto-completion instructions based on current skill state.
func BuildSkillCompletionHints(role string, skills []TeamMemberSkill) string {
	var hints []string

	if len(skills) == 0 {
		hints = append(hints, fmt.Sprintf(
			"得意分野（skills）が未設定です。あなたの役割「%s」に基づき、register_agent 実行時に適切な得意分野を3〜5件、name と description 付きで追加してください。",
			role,
		))
	} else {
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

var agentNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// DeriveAgentNames generates unique agent names from member pane titles.
func DeriveAgentNames(members []TeamMember) map[string]string {
	result := make(map[string]string, len(members))
	used := make(map[string]int, len(members))
	for _, member := range members {
		base := sanitizeAgentName(member.PaneTitle)
		used[base]++
		name := base
		if used[base] > 1 {
			name = fmt.Sprintf("%s-%d", base, used[base])
		}
		result[member.ID] = name
	}
	return result
}

func sanitizeAgentName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = agentNameSanitizer.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "member"
	}
	return normalized
}

func cloneAndSortPanes(panes []tmux.PaneSnapshot) []tmux.PaneSnapshot {
	cloned := append([]tmux.PaneSnapshot{}, panes...)
	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].Index != cloned[j].Index {
			return cloned[i].Index < cloned[j].Index
		}
		return cloned[i].ID < cloned[j].ID
	})
	return cloned
}

func findTeamByID(teams []TeamDefinition, teamID string) (TeamDefinition, error) {
	for _, team := range teams {
		if team.ID == teamID {
			return team, nil
		}
	}
	return TeamDefinition{}, fmt.Errorf("team %s not found", teamID)
}
