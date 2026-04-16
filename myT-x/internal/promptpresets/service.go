package promptpresets

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"myT-x/internal/config"
	"myT-x/internal/orchestrator"
	"myT-x/internal/tmux"
)

type Deps struct {
	ConfigPath          func() string
	FindSessionSnapshot func(sessionName string) (tmux.SessionSnapshot, error)
}

type Service struct {
	deps Deps
	mu   sync.Mutex
}

func NewService(deps Deps) *Service {
	if deps.ConfigPath == nil || deps.FindSessionSnapshot == nil {
		panic("promptpresets.NewService: required function fields in Deps must be non-nil (ConfigPath, FindSessionSnapshot)")
	}
	return &Service{deps: deps}
}

func (s *Service) Save(preset PromptPreset, sessionName string) error {
	storageLocation, err := normalizeStorageLocation(preset.StorageLocation)
	if err != nil {
		return err
	}

	preset.StorageLocation = ""
	preset.Normalize()
	if err := preset.Validate(); err != nil {
		return err
	}

	path, err := s.resolveStoragePath(storageLocation, sessionName)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	presets, err := readPresetsForWrite(path)
	if err != nil {
		return fmt.Errorf("read prompt presets: %w", err)
	}

	presets, err = upsertPreset(presets, preset)
	if err != nil {
		return err
	}
	sortPresets(presets)

	if err := writePresets(path, presets); err != nil {
		return fmt.Errorf("write prompt presets: %w", err)
	}
	return nil
}

func (s *Service) Load(sessionName string) (LoadResult, error) {
	globalPath, err := s.resolveStoragePath(StorageLocationGlobal, "")
	if err != nil {
		return LoadResult{}, err
	}
	globalPresets, globalWarning, err := readPresets(globalPath)
	if err != nil {
		return LoadResult{}, fmt.Errorf("read global prompt presets: %w", err)
	}
	sortPresets(globalPresets)
	assignStorageLocation(globalPresets, StorageLocationGlobal)
	warnings := make([]string, 0, 2)
	appendPromptPresetWarning(&warnings, globalWarning)

	if strings.TrimSpace(sessionName) == "" {
		return LoadResult{Presets: globalPresets, Warnings: warnings}, nil
	}

	projectPath, err := s.resolveStoragePath(StorageLocationProject, sessionName)
	if err != nil {
		slog.Warn("[WARN-PROMPT-PRESETS] failed to resolve project prompt presets path, returning global presets only",
			"session", strings.TrimSpace(sessionName),
			"error", err,
		)
		appendPromptPresetWarning(&warnings, fmt.Sprintf("Project prompt presets could not be resolved for session %q. Showing global presets only.", strings.TrimSpace(sessionName)))
		return LoadResult{Presets: globalPresets, Warnings: warnings}, nil
	}
	projectPresets, projectWarning, err := readPresets(projectPath)
	if err != nil {
		slog.Warn("[WARN-PROMPT-PRESETS] failed to read project prompt presets, returning global presets only",
			"session", strings.TrimSpace(sessionName),
			"path", projectPath,
			"error", err,
		)
		appendPromptPresetWarning(&warnings, fmt.Sprintf("Project prompt presets could not be loaded from %s. Showing global presets only.", projectPath))
		return LoadResult{Presets: globalPresets, Warnings: warnings}, nil
	}
	sortPresets(projectPresets)
	assignStorageLocation(projectPresets, StorageLocationProject)
	appendPromptPresetWarning(&warnings, projectWarning)

	result := make([]PromptPreset, 0, len(globalPresets)+len(projectPresets))
	result = append(result, globalPresets...)
	result = append(result, projectPresets...)
	return LoadResult{Presets: result, Warnings: warnings}, nil
}

func (s *Service) Delete(id, storageLocation, sessionName string) error {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return errors.New("prompt preset id is required")
	}

	normalizedLocation, err := normalizeStorageLocation(storageLocation)
	if err != nil {
		return err
	}
	path, err := s.resolveStoragePath(normalizedLocation, sessionName)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	presets, err := readPresetsForWrite(path)
	if err != nil {
		return fmt.Errorf("read prompt presets: %w", err)
	}

	filtered := presets[:0]
	found := false
	for _, preset := range presets {
		if preset.ID == trimmedID {
			found = true
			continue
		}
		filtered = append(filtered, preset)
	}
	if !found {
		return nil
	}

	resequenceOrders(filtered)
	sortPresets(filtered)
	if err := writePresets(path, filtered); err != nil {
		return fmt.Errorf("write prompt presets: %w", err)
	}
	return nil
}

func (s *Service) Reorder(ids []string, storageLocation, sessionName string) error {
	normalizedLocation, err := normalizeStorageLocation(storageLocation)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	path, err := s.resolveStoragePath(normalizedLocation, sessionName)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	presets, err := readPresetsForWrite(path)
	if err != nil {
		return fmt.Errorf("read prompt presets: %w", err)
	}
	if len(presets) == 0 {
		return fmt.Errorf("prompt preset %q not found", strings.TrimSpace(ids[0]))
	}

	reordered, err := reorderPresets(presets, ids)
	if err != nil {
		return err
	}
	sortPresets(reordered)

	if err := writePresets(path, reordered); err != nil {
		return fmt.Errorf("write prompt presets: %w", err)
	}
	return nil
}

func (s *Service) resolveStoragePath(storageLocation, sessionName string) (string, error) {
	switch storageLocation {
	case StorageLocationGlobal:
		return s.resolveGlobalStoragePath(), nil
	case StorageLocationProject:
		return s.resolveProjectStoragePath(sessionName)
	default:
		return "", fmt.Errorf("unsupported prompt preset storage location: %s", storageLocation)
	}
}

func (s *Service) resolveGlobalStoragePath() string {
	configPath := strings.TrimSpace(s.deps.ConfigPath())
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	return filepath.Join(filepath.Dir(configPath), presetsFileName)
}

func (s *Service) resolveProjectStoragePath(sessionName string) (string, error) {
	trimmedSessionName := strings.TrimSpace(sessionName)
	if trimmedSessionName == "" {
		return "", errors.New("session name is required for project prompt presets")
	}

	snapshot, err := s.deps.FindSessionSnapshot(trimmedSessionName)
	if err != nil {
		return "", err
	}
	rootPath, err := orchestrator.ResolveSourceRootPath(snapshot)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, ".myT-x", presetsFileName), nil
}

func normalizeStorageLocation(storageLocation string) (string, error) {
	trimmed := strings.TrimSpace(storageLocation)
	if trimmed == "" {
		return StorageLocationGlobal, nil
	}
	switch trimmed {
	case StorageLocationGlobal, StorageLocationProject:
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported prompt preset storage location: %s", trimmed)
	}
}

func assignStorageLocation(presets []PromptPreset, storageLocation string) {
	for index := range presets {
		presets[index].StorageLocation = storageLocation
	}
}

func upsertPreset(existing []PromptPreset, preset PromptPreset) ([]PromptPreset, error) {
	for index := range existing {
		if existing[index].ID != preset.ID {
			continue
		}
		existing[index].Name = preset.Name
		existing[index].Body = preset.Body
		return existing, nil
	}

	if len(existing) >= MaxPresets {
		return nil, fmt.Errorf("prompt presets must be %d or fewer", MaxPresets)
	}

	preset.Order = len(existing)
	return append(existing, preset), nil
}

func reorderPresets(existing []PromptPreset, ids []string) ([]PromptPreset, error) {
	seen := make(map[string]struct{}, len(ids))
	byID := make(map[string]PromptPreset, len(existing))
	for _, preset := range existing {
		byID[preset.ID] = preset
	}

	reordered := make([]PromptPreset, 0, len(existing))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			return nil, errors.New("prompt preset id is required for reorder")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("duplicate prompt preset id in reorder: %s", id)
		}
		preset, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("prompt preset %q not found", id)
		}
		seen[id] = struct{}{}
		reordered = append(reordered, preset)
	}

	for _, preset := range existing {
		if _, ok := seen[preset.ID]; ok {
			continue
		}
		reordered = append(reordered, preset)
	}

	resequenceOrders(reordered)
	return reordered, nil
}

func resequenceOrders(presets []PromptPreset) {
	for index := range presets {
		presets[index].Order = index
	}
}

func sortPresets(presets []PromptPreset) {
	sort.SliceStable(presets, func(i, j int) bool {
		if presets[i].Order != presets[j].Order {
			return presets[i].Order < presets[j].Order
		}
		left := strings.ToLower(presets[i].Name)
		right := strings.ToLower(presets[j].Name)
		if left != right {
			return left < right
		}
		return presets[i].ID < presets[j].ID
	})
}

func appendPromptPresetWarning(warnings *[]string, warning string) {
	if strings.TrimSpace(warning) == "" {
		return
	}
	*warnings = append(*warnings, warning)
}

func readPresets(path string) ([]PromptPreset, string, error) {
	return readPresetsWithMode(path, true)
}

func readPresetsForWrite(path string) ([]PromptPreset, error) {
	presets, _, err := readPresetsWithMode(path, false)
	return presets, err
}

func readPresetsWithMode(path string, allowMalformed bool) ([]PromptPreset, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []PromptPreset{}, "", nil
		}
		return nil, "", err
	}

	var presets []PromptPreset
	if err := json.Unmarshal(data, &presets); err != nil {
		backupPresetsFile(path, data)
		if allowMalformed {
			slog.Debug("[DEBUG-PROMPT-PRESETS] failed to parse prompt presets, returning empty", "path", path, "error", err)
			return []PromptPreset{}, fmt.Sprintf("Prompt presets at %s could not be parsed. Showing an empty list for that scope.", path), nil
		}
		slog.Warn("[WARN-PROMPT-PRESETS] failed to parse prompt presets, refusing to overwrite", "path", path, "error", err)
		return nil, "", fmt.Errorf("parse prompt presets: %w", err)
	}

	for index := range presets {
		presets[index].Normalize()
	}
	return presets, "", nil
}

func backupPresetsFile(path string, data []byte) {
	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		slog.Warn("[WARN-PROMPT-PRESETS] failed to create backup of malformed file", "path", backupPath, "error", err)
		return
	}
	slog.Info("[WARN-PROMPT-PRESETS] created backup of malformed file", "path", backupPath)
}

func writePresets(path string, presets []PromptPreset) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create prompt preset directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(presets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prompt presets: %w", err)
	}
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(dir, ".prompt-presets.*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for prompt presets: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
		}
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			if removeErr := os.Remove(tmpPath); removeErr != nil {
				slog.Debug("[DEBUG-PROMPT-PRESETS] failed to remove temp file", "path", tmpPath, "error", removeErr)
			}
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		return fmt.Errorf("write prompt presets: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync prompt presets temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close prompt presets temp file: %w", err)
	}
	tmpFile = nil

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace prompt presets file: %w", err)
	}
	return nil
}
