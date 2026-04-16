package usagedashboard

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"myT-x/internal/usagedashboard/claude"
	"myT-x/internal/usagedashboard/codex"
)

// Mode selects which source(s) to aggregate.
const (
	ModeClaude = "claude"
	ModeCodex  = "codex"
	ModeBoth   = "both"
)

// Deps are required external dependencies injected by the composition root.
// ResolveSessionWorkDir must be non-nil; the rest fall back to safe defaults.
type Deps struct {
	// ResolveSessionWorkDir maps a session name to its effective working
	// directory. Worktree sessions must return Worktree.Path, regular sessions
	// must return RootPath. See internal/session/service.go:ResolveSessionWorkDir.
	ResolveSessionWorkDir func(sessionName string) (string, error)

	// HomeDir returns the user home directory. Defaults to os.UserHomeDir.
	HomeDir func() (string, error)

	// NowFunc returns the current time. Defaults to time.Now.
	NowFunc func() time.Time

	// SnapshotRepo persists the aggregation result to disk so subsequent
	// dashboard opens can return cached data instead of re-scanning every
	// JSONL file. nil → defaults to NewFileSnapshotRepository().
	SnapshotRepo SnapshotRepository

	// SnapshotTTL is how long a cached snapshot is considered fresh.
	// 0 → DefaultSnapshotTTL (24h).
	SnapshotTTL time.Duration
}

// Service aggregates Claude/Codex usage statistics scoped to one session folder.
// All fields are safe for concurrent use.
type Service struct {
	deps Deps
	mu   sync.Mutex // serializes cache repository access
}

// NewService constructs the service. Panics when required Deps are nil
// (defensive-coding-checklist #180).
func NewService(deps Deps) *Service {
	if deps.ResolveSessionWorkDir == nil {
		panic("usagedashboard.NewService: Deps.ResolveSessionWorkDir is required")
	}
	if deps.HomeDir == nil {
		deps.HomeDir = os.UserHomeDir
	}
	if deps.NowFunc == nil {
		deps.NowFunc = time.Now
	}
	if deps.SnapshotRepo == nil {
		deps.SnapshotRepo = NewFileSnapshotRepository()
	}
	if deps.SnapshotTTL <= 0 {
		deps.SnapshotTTL = DefaultSnapshotTTL
	}
	return &Service{deps: deps}
}

// GetUsageDashboard returns aggregated usage statistics for sessionName,
// served from the per-project JSON cache when fresh.
//
// When force is false:
//   - A cached snapshot at <workDir>/.myT-x/usage-dashboard.json is returned
//     when present and within Deps.SnapshotTTL. Read errors and schema
//     mismatches are logged and treated as cache miss (re-aggregate).
//
// When force is true (e.g. user pressed "Refresh" in the UI):
//   - The cache is bypassed, both sources are re-scanned, and the file is
//     overwritten with the fresh result.
//
// Aggregation always covers BOTH Claude and Codex regardless of mode so
// the saved file is mode-agnostic and a later mode switch never triggers
// an unnecessary re-aggregation. The returned snapshot is filtered to the
// requested mode in filterByMode.
//
// Contract (#157):
//   - error==nil implies LastUpdatedAt is non-zero.
//   - error==nil implies PartialErrors is a non-nil (possibly empty) slice
//     on every returned SourceHealth.
//   - mode="claude" → Claude!=nil, Codex==nil
//   - mode="codex"  → Claude==nil, Codex!=nil
//   - mode="both"   → both non-nil
func (s *Service) GetUsageDashboard(sessionName, mode string, force bool) (UsageDashboardSnapshot, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return UsageDashboardSnapshot{}, errors.New("session name is empty")
	}
	mode = normalizeMode(mode)

	workDir, err := s.deps.ResolveSessionWorkDir(sessionName)
	if err != nil {
		return UsageDashboardSnapshot{}, fmt.Errorf("resolve session work dir: %w", err)
	}
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return UsageDashboardSnapshot{}, fmt.Errorf("session %q has no working directory", sessionName)
	}

	homeDir, err := s.deps.HomeDir()
	if err != nil {
		return UsageDashboardSnapshot{}, fmt.Errorf("resolve user home: %w", err)
	}

	now := s.deps.NowFunc().UTC()

	if !force {
		s.mu.Lock()
		cached, ok := s.tryLoadCache(workDir, now)
		s.mu.Unlock()
		if ok {
			return filterByMode(cached, mode), nil
		}
	}

	persisted := s.aggregateBoth(homeDir, workDir, now)

	s.mu.Lock()
	err = s.deps.SnapshotRepo.Save(persisted)
	s.mu.Unlock()
	if err != nil {
		// Persistence failures must not block returning fresh data.
		// Falling through here keeps the dashboard usable even when
		// the project directory is read-only.
		slog.Warn("[USAGE_DASHBOARD_DEBUG] snapshot save failed",
			"work_dir", workDir, "err", err)
	}

	return filterByMode(persisted, mode), nil
}

// tryLoadCache returns the persisted snapshot when it exists, parses
// successfully, and is still within the TTL window. JSON read/parse
// errors are logged and treated as a miss so the caller will re-aggregate
// and overwrite the corrupt file.
func (s *Service) tryLoadCache(workDir string, now time.Time) (PersistedSnapshot, bool) {
	snap, found, err := s.deps.SnapshotRepo.Load(workDir)
	if err != nil {
		slog.Warn("[USAGE_DASHBOARD_DEBUG] snapshot load failed, will re-aggregate",
			"work_dir", workDir, "err", err)
		return PersistedSnapshot{}, false
	}
	if !found {
		return PersistedSnapshot{}, false
	}
	if isExpired(snap.SavedAt, now, s.deps.SnapshotTTL) {
		return PersistedSnapshot{}, false
	}
	if snap.Claude == nil || snap.Codex == nil {
		// Defensive: the writer always populates both. A partial cache
		// usually means a previous version wrote it, so re-aggregate.
		return PersistedSnapshot{}, false
	}
	normalizeCachedSnapshot(&snap)
	return snap, true
}

func normalizeCachedSnapshot(snap *PersistedSnapshot) {
	if snap == nil {
		return
	}
	normalizeClaudeStats(snap.Claude)
	normalizeCodexStats(snap.Codex)
}

func normalizeClaudeStats(stats *ClaudeUsageStats) {
	if stats == nil {
		return
	}
	if stats.Skills == nil {
		stats.Skills = []UsageEntry{}
	}
	if stats.Agents == nil {
		stats.Agents = []UsageEntry{}
	}
	if stats.SlashCommands == nil {
		stats.SlashCommands = []UsageEntry{}
	}
	if stats.DailyActivity == nil {
		stats.DailyActivity = []DailyBucket{}
	}
	if stats.Health.PartialErrors == nil {
		stats.Health.PartialErrors = []string{}
	}
}

func normalizeCodexStats(stats *CodexUsageStats) {
	if stats == nil {
		return
	}
	if stats.Skills == nil {
		stats.Skills = []UsageEntry{}
	}
	if stats.Agents == nil {
		stats.Agents = []UsageEntry{}
	}
	if stats.DailyActivity == nil {
		stats.DailyActivity = []DailyBucket{}
	}
	if stats.Health.PartialErrors == nil {
		stats.Health.PartialErrors = []string{}
	}
}

// aggregateBoth runs Claude and Codex aggregation in parallel and
// returns a fully populated PersistedSnapshot ready to be saved.
func (s *Service) aggregateBoth(homeDir, workDir string, now time.Time) PersistedSnapshot {
	var wg sync.WaitGroup
	var claudeStats *ClaudeUsageStats
	var codexStats *CodexUsageStats
	wg.Go(func() {
		claudeStats = s.buildClaudeStats(homeDir, workDir)
	})
	wg.Go(func() {
		codexStats = s.buildCodexStats(homeDir, workDir)
	})
	wg.Wait()

	return PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now,
		Claude:        claudeStats,
		Codex:         codexStats,
	}
}

// filterByMode converts a fully-populated persisted snapshot into the API
// response shape required by the requested mode (#157 contract).
func filterByMode(p PersistedSnapshot, mode string) UsageDashboardSnapshot {
	snap := UsageDashboardSnapshot{
		WorkDir:       p.WorkDir,
		LastUpdatedAt: p.SavedAt,
	}
	if mode == ModeClaude || mode == ModeBoth {
		snap.Claude = p.Claude
	}
	if mode == ModeCodex || mode == ModeBoth {
		snap.Codex = p.Codex
	}
	return snap
}

func normalizeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModeClaude:
		return ModeClaude
	case ModeCodex:
		return ModeCodex
	default:
		return ModeBoth
	}
}

// buildClaudeStats is always non-nil so the mode contract holds.
func (s *Service) buildClaudeStats(homeDir, workDir string) *ClaudeUsageStats {
	stats := &ClaudeUsageStats{
		Skills:        []UsageEntry{},
		Agents:        []UsageEntry{},
		SlashCommands: []UsageEntry{},
		DailyActivity: []DailyBucket{},
		Health:        newSourceHealth(),
	}
	claudeHome, err := ResolveClaudeHome(homeDir)
	if err != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, err.Error())
		stats.DailyActivity = NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow).Buckets()
		return stats
	}
	projectDir, err := FindClaudeProjectDir(claudeHome, workDir)
	if err != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, err.Error())
		stats.DailyActivity = NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow).Buckets()
		return stats
	}
	stats.Health.ProjectDir = projectDir

	paths, err := claude.ListProjectFiles(projectDir)
	if err != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("list project files: %s", err.Error()))
		stats.DailyActivity = NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow).Buckets()
		return stats
	}
	stats.Health.JsonlAvailable = len(paths) > 0

	skills := NewUsageCounter()
	agents := NewUsageCounter()
	slash := NewUsageCounter()
	daily := NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow)
	sessionsSeen := make(map[string]struct{})
	for _, path := range paths {
		session, err := claude.ParseFile(path, &stats.Health.PartialErrors)
		if err != nil {
			stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("%s: %s", filepath.Base(path), err.Error()))
			continue
		}
		if session.SessionID != "" {
			if _, ok := sessionsSeen[session.SessionID]; !ok {
				sessionsSeen[session.SessionID] = struct{}{}
				if !session.FirstSeen.IsZero() {
					daily.AddSession(session.FirstSeen)
				}
			}
		}
		for _, rec := range session.Records {
			applyClaudeRecord(rec, stats, skills, agents, slash, daily, true)
		}
	}

	// Walk subagent logs (<session-uuid>/subagents/*.jsonl) to capture Skill
	// and Agent uses that happened inside spawned subagents. Sessions and
	// messages are NOT counted here because the parent session's tool_use
	// already accounts for the subagent invocation; double-counting would
	// inflate TotalSessions/TotalMessages artificially.
	subagentPaths, subErr := claude.ListSubagentFiles(projectDir)
	if subErr != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("list subagent files: %s", subErr.Error()))
	}
	for _, path := range subagentPaths {
		session, err := claude.ParseFile(path, &stats.Health.PartialErrors)
		if err != nil {
			stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("%s: %s", filepath.Base(path), err.Error()))
			continue
		}
		for _, rec := range session.Records {
			applyClaudeRecord(rec, stats, skills, agents, slash, daily, false)
		}
	}

	stats.TotalSessions = len(sessionsSeen)
	stats.Skills = skills.TopN(TopRankingLimit)
	stats.Agents = agents.TopN(TopRankingLimit)
	stats.SlashCommands = slash.TopN(TopRankingLimit)
	stats.DailyActivity = daily.Buckets()
	stats.ActiveDays = daily.ActiveDays()
	// history.jsonl presence is used as an indicator only.
	if _, herr := os.Stat(filepath.Join(claudeHome, "history.jsonl")); herr == nil {
		stats.Health.HistoryAvailable = true
	}
	return stats
}

func (s *Service) buildCodexStats(homeDir, workDir string) *CodexUsageStats {
	stats := &CodexUsageStats{
		Skills:        []UsageEntry{},
		Agents:        []UsageEntry{},
		DailyActivity: []DailyBucket{},
		Health:        newSourceHealth(),
	}
	codexHome, err := ResolveCodexHome(homeDir)
	if err != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, err.Error())
		stats.DailyActivity = NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow).Buckets()
		return stats
	}
	sessionsRoot := filepath.Join(codexHome, "sessions")
	stats.Health.ProjectDir = sessionsRoot
	paths, err := codex.ListRolloutFiles(sessionsRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Debug("[USAGE_DASHBOARD_DEBUG] codex sessions root not found",
				"path", sessionsRoot)
		} else {
			slog.Warn("[USAGE_DASHBOARD_DEBUG] codex list rollout files failed",
				"path", sessionsRoot, "err", err)
			stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("list codex sessions: %s", err.Error()))
		}
	}
	if err == nil {
		stats.Health.JsonlAvailable = len(paths) > 0
	}

	agents := NewUsageCounter()
	skills := NewUsageCounter()
	daily := NewDailyAggregator(s.deps.NowFunc(), DefaultDailyWindow)
	sessionsSeen := make(map[string]struct{})
	matcher := func(cwd string) bool { return PathsEqualFold(cwd, workDir) }

	for _, path := range paths {
		session, matched, err := codex.ParseFile(path, matcher, &stats.Health.PartialErrors)
		if err != nil {
			stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("%s: %s", filepath.Base(path), err.Error()))
			continue
		}
		if !matched {
			continue
		}
		if session.SessionID != "" {
			if _, ok := sessionsSeen[session.SessionID]; !ok {
				sessionsSeen[session.SessionID] = struct{}{}
				if !session.FirstSeen.IsZero() {
					daily.AddSession(session.FirstSeen)
				}
			}
		}
		for _, rec := range session.Records {
			switch rec.Type {
			case codex.RecordSpawnAgent:
				agents.Add(rec.Name, rec.Timestamp)
				stats.TotalSpawnedAgents++
				daily.AddToolCall(rec.Timestamp)
			case codex.RecordSkillRead:
				skills.Add(rec.Name, rec.Timestamp)
			case codex.RecordUserPrompt:
				stats.TotalPrompts++
				daily.AddSecondary(rec.Timestamp)
			case codex.RecordAssistantMessage:
				// Assistant messages are counted in session metadata only and
				// do not contribute to ranking or daily buckets (reserved for
				// future token/latency metrics).
			}
		}
	}
	stats.TotalSessions = len(sessionsSeen)
	stats.Skills = skills.TopN(TopRankingLimit)
	stats.Agents = agents.TopN(TopRankingLimit)
	stats.DailyActivity = daily.Buckets()
	stats.ActiveDays = daily.ActiveDays()

	// SQLite read (best-effort).
	// Unlike Claude, Codex has a second data source (state_5.sqlite), so
	// rollout discovery failures remain non-fatal and we still attempt the
	// SQLite summary to surface whatever data is available.
	sqlitePath := filepath.Join(codexHome, "state_5.sqlite")
	summary, sqliteErr := codex.ReadSQLite(sqlitePath, workDir)
	if sqliteErr != nil {
		stats.Health.PartialErrors = append(stats.Health.PartialErrors, fmt.Sprintf("sqlite: %s", sqliteErr.Error()))
	} else {
		stats.Health.SqliteAvailable = summary.Available
		if summary.Available {
			// Prefer SQLite thread count when it is higher (jsonl may be incomplete).
			if summary.TotalThreads > stats.TotalSessions {
				stats.TotalSessions = summary.TotalThreads
			}
			if summary.ActiveDays > stats.ActiveDays {
				stats.ActiveDays = summary.ActiveDays
			}
		}
	}
	if _, herr := os.Stat(filepath.Join(codexHome, "history.jsonl")); herr == nil {
		stats.Health.HistoryAvailable = true
	}
	return stats
}

func newSourceHealth() SourceHealth {
	return SourceHealth{PartialErrors: []string{}}
}

// applyClaudeRecord aggregates one parsed record into the running Claude
// counters. Extracted to keep main-session and subagent loops in sync when new
// RecordType values are added in the future (defensive-coding-checklist #187).
//
// countMessages controls whether user/assistant messages contribute to
// TotalMessages and daily.Secondary. Subagent files pass false because the
// parent session already accounts for message volume; counting them again
// would inflate metrics.
func applyClaudeRecord(
	rec claude.Record,
	stats *ClaudeUsageStats,
	skills, agents, slash *UsageCounter,
	daily *DailyAggregator,
	countMessages bool,
) {
	switch rec.Type {
	case claude.RecordSkill:
		skills.Add(rec.Name, rec.Timestamp)
		stats.TotalToolUses++
		daily.AddToolCall(rec.Timestamp)
	case claude.RecordAgent:
		agents.Add(rec.Name, rec.Timestamp)
		stats.TotalToolUses++
		daily.AddToolCall(rec.Timestamp)
	case claude.RecordSlash:
		slash.Add(rec.Name, rec.Timestamp)
	case claude.RecordUserMessage, claude.RecordAssistantMessage:
		if countMessages {
			stats.TotalMessages++
			daily.AddSecondary(rec.Timestamp)
		}
	case claude.RecordToolCall:
		stats.TotalToolUses++
		daily.AddToolCall(rec.Timestamp)
	}
}
