package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	sample_teams "myT-x/embed/sample_teams"
	"myT-x/internal/config"
)

// seedOrchestratorTeamSamplesOnce ensures sample teams are seeded at most once
// per process lifetime, regardless of concurrent calls.
var seedOrchestratorTeamSamplesOnce sync.Once

const (
	orchestratorTeamDefinitionsFileName = "orchestrator-team-definitions.json"
	orchestratorTeamMembersFileName     = "orchestrator-team-members.json"
)

type orchestratorTeamFileRecord struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Order            int    `json:"order"`
	BootstrapDelayMs int    `json:"bootstrap_delay_ms,omitempty"`
}

// SaveOrchestratorTeam はチーム定義を保存（新規作成または更新）する。
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

	// Two-file write strategy:
	// - If definitions write succeeds but members write fails: orphan members from
	//   a previous state may be stale, but filterOrchestratorOrphanMembers cleans
	//   them up on next load. The new definition exists without its members.
	// - If definitions write fails: we return early without updating members,
	//   preserving the previous consistent state.
	if err := writeOrchestratorTeamDefinitions(definitionsPath, definitions); err != nil {
		return err
	}
	if err := writeOrchestratorTeamMembers(membersPath, members); err != nil {
		return err
	}
	return nil
}

// LoadOrchestratorTeams はグローバルおよびプロジェクトローカルのチーム定義を読み込む。
func (a *App) LoadOrchestratorTeams(sessionName string) ([]OrchestratorTeamDefinition, error) {
	definitionsPath, membersPath := a.resolveOrchestratorTeamStoragePaths()

	// Seed sample teams outside the lock (file I/O may be slow).
	// seedOrchestratorTeamSamples is idempotent (checks file existence).
	seedOrchestratorTeamSamples(definitionsPath, membersPath)

	a.orchestratorTeamMu.Lock()
	defer a.orchestratorTeamMu.Unlock()

	definitions, err := readOrchestratorTeamDefinitions(definitionsPath)
	if err != nil {
		return []OrchestratorTeamDefinition{}, fmt.Errorf("read team definitions: %w", err)
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

// DeleteOrchestratorTeam は指定されたチーム定義を削除する。
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

	// Two-file write strategy:
	// - If definitions write succeeds but members write fails: the deleted team's
	//   members remain but are cleaned up by filterOrchestratorOrphanMembers on next load.
	// - If definitions write fails: we return early without updating members,
	//   preserving the previous consistent state.
	if err := writeOrchestratorTeamDefinitions(definitionsPath, filteredDefinitions); err != nil {
		return err
	}
	if err := writeOrchestratorTeamMembers(membersPath, filteredMembers); err != nil {
		return err
	}
	return nil
}

// ReorderOrchestratorTeams は指定された順序でチーム定義を並び替える。
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
		// Create a .bak backup before potentially losing the malformed data.
		backupOrchestratorTeamFile(path, data)
		if allowMalformed {
			slog.Debug("[DEBUG-ORCH-TEAM] failed to parse team definitions, returning empty", "path", path, "error", err)
			return []orchestratorTeamFileRecord{}, nil
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
		backupOrchestratorTeamFile(path, data)
		if allowMalformed {
			slog.Debug("[DEBUG-ORCH-TEAM] failed to parse team members, returning empty", "path", path, "error", err)
			return []OrchestratorTeamMember{}, nil
		}
		slog.Warn("[WARN-ORCH-TEAM] failed to parse team members, refusing to overwrite", "path", path, "error", err)
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

// backupOrchestratorTeamFile はパース失敗時にデータ損失を防ぐため .bak ファイルを作成する。
// best-effort: バックアップ作成失敗はログ出力のみで続行する。
func backupOrchestratorTeamFile(path string, data []byte) {
	bakPath := path + ".bak"
	if err := os.WriteFile(bakPath, data, 0o644); err != nil {
		slog.Warn("[WARN-ORCH-TEAM] failed to create backup of malformed file", "path", bakPath, "error", err)
	} else {
		slog.Info("[WARN-ORCH-TEAM] created backup of malformed file", "path", bakPath)
	}
}

const orchestratorTeamRenameRetryMax = 10
const orchestratorTeamRenameRetryBaseDelay = 10 * time.Millisecond

func writeOrchestratorTeamJSON(path string, payload any, label string) error {
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
	for attempt := range orchestratorTeamRenameRetryMax {
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
		time.Sleep(time.Duration(attempt+1) * orchestratorTeamRenameRetryBaseDelay)
	}
	return lastErr
}

// seedOrchestratorTeamSamples は定義ファイルが未存在の場合に埋め込みサンプルを書き出す。
// ユーザーが全チーム削除後（ファイルは存在するが空配列）には再シードしない。
// sync.Once により1プロセスにつき1回のみ実行される。
func seedOrchestratorTeamSamples(definitionsPath, membersPath string) {
	seedOrchestratorTeamSamplesOnce.Do(func() {
		seedOrchestratorTeamSamplesInternal(definitionsPath, membersPath)
	})
}

func seedOrchestratorTeamSamplesInternal(definitionsPath, membersPath string) bool {
	// ファイルが既に存在する場合はシードしない
	if _, err := os.Stat(definitionsPath); err == nil {
		return false
	} else if !os.IsNotExist(err) {
		slog.Debug("[DEBUG-ORCH-TEAM] cannot check sample definitions file", "path", definitionsPath, "error", err)
		return false
	}

	defsData, err := sample_teams.FS.ReadFile(orchestratorTeamDefinitionsFileName)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to read embedded sample definitions", "error", err)
		return false
	}
	membersData, err := sample_teams.FS.ReadFile(orchestratorTeamMembersFileName)
	if err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to read embedded sample members", "error", err)
		return false
	}

	// Parse embedded data to use writeOrchestratorTeamJSON for atomic writes.
	var members []OrchestratorTeamMember
	if err := json.Unmarshal(membersData, &members); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to parse embedded sample members", "error", err)
		return false
	}
	var definitions []orchestratorTeamFileRecord
	if err := json.Unmarshal(defsData, &definitions); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to parse embedded sample definitions", "error", err)
		return false
	}

	// Write members first, then definitions. The definitions file is used as the
	// existence check for seeding, so writing it last ensures that if the process
	// is interrupted after members are written, the next startup will re-seed both.
	if err := writeOrchestratorTeamMembers(membersPath, members); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to write sample members", "error", err)
		return false
	}
	if err := writeOrchestratorTeamDefinitions(definitionsPath, definitions); err != nil {
		slog.Debug("[DEBUG-ORCH-TEAM] failed to write sample definitions", "error", err)
		return false
	}
	slog.Info("[DEBUG-ORCH-TEAM] seeded sample team definitions for first-time use")
	return true
}
