// Package usagedashboard aggregates read-only usage statistics for Claude Code
// and Codex sessions scoped to the current session folder.
//
// The package is split into three layers:
//   - types.go/paths.go/aggregator.go: shared DTOs and utilities
//   - claude/: Claude Code JSONL parser (~/.claude/projects/<slug>/*.jsonl)
//   - codex/: Codex JSONL + SQLite reader (~/.codex/sessions, ~/.codex/state_5.sqlite)
//
// Source priority is read-only; parse failures are classified as either expected
// (skipped silently) or unexpected (appended to SourceHealth.PartialErrors).
package usagedashboard

import "time"

// DefaultDailyWindow is the rolling activity window in days.
const DefaultDailyWindow = 30

// TopRankingLimit caps the number of entries returned per ranking table.
const TopRankingLimit = 20

// UsageDashboardSnapshot is the top-level DTO returned by GetUsageDashboard.
//
// Mode semantics (see service.go):
//   - mode="claude": Claude non-nil, Codex nil
//   - mode="codex":  Claude nil,     Codex non-nil
//   - mode="both":   Claude non-nil, Codex non-nil
//
// LastUpdatedAt is always set to the time the snapshot finished aggregating.
type UsageDashboardSnapshot struct {
	Claude        *ClaudeUsageStats `json:"claude,omitempty"`
	Codex         *CodexUsageStats  `json:"codex,omitempty"`
	LastUpdatedAt time.Time         `json:"last_updated_at"`
	WorkDir       string            `json:"work_dir"`
}

// ClaudeUsageStats is the Claude Code aggregation result.
// All slice fields are guaranteed non-nil (empty slice) when error==nil.
type ClaudeUsageStats struct {
	TotalSessions      int                `json:"total_sessions"`
	ActiveDays         int                `json:"active_days"`
	TotalMessages      int                `json:"total_messages"`
	TotalToolUses      int                `json:"total_tool_uses"`
	Skills             []UsageEntry       `json:"skills"`
	Agents             []UsageEntry       `json:"agents"`
	SlashCommands      []UsageEntry       `json:"slash_commands"`
	SkillsDaily        []DailyUsageSeries `json:"skills_daily"`
	AgentsDaily        []DailyUsageSeries `json:"agents_daily"`
	SlashCommandsDaily []DailyUsageSeries `json:"slash_commands_daily"`
	DailyActivity      []DailyBucket      `json:"daily_activity"`
	Health             SourceHealth       `json:"health"`
}

// CodexUsageStats is the Codex aggregation result.
// All slice fields are guaranteed non-nil (empty slice) when error==nil.
type CodexUsageStats struct {
	TotalSessions      int                `json:"total_sessions"`
	ActiveDays         int                `json:"active_days"`
	TotalPrompts       int                `json:"total_prompts"`
	TotalSpawnedAgents int                `json:"total_spawned_agents"`
	Skills             []UsageEntry       `json:"skills"`
	Agents             []UsageEntry       `json:"agents"`
	SkillsDaily        []DailyUsageSeries `json:"skills_daily"`
	AgentsDaily        []DailyUsageSeries `json:"agents_daily"`
	DailyActivity      []DailyBucket      `json:"daily_activity"`
	Health             SourceHealth       `json:"health"`
}

// UsageEntry is a generic ranking row (skill/agent/slash command).
type UsageEntry struct {
	Name       string    `json:"name"`
	Count      int       `json:"count"`
	LastUsedAt time.Time `json:"last_used_at"`
}

// DailyUsageSeries is a named 30-day usage series for an item included in the
// matching ranking payload.
type DailyUsageSeries struct {
	Name       string             `json:"name"`
	TotalCount int                `json:"total_count"`
	LastUsedAt time.Time          `json:"last_used_at"`
	Buckets    []DailyUsageBucket `json:"buckets"`
}

// DailyUsageBucket is one item's count for a UTC day.
type DailyUsageBucket struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// DailyBucket is a single day of activity.
//
// The three counter columns are source-generic; the frontend attaches labels:
//   - Claude: Sessions, Messages,  ToolCalls
//   - Codex:  Sessions, Prompts,   SpawnedAgents
//
// Date is "YYYY-MM-DD" in UTC.
type DailyBucket struct {
	Date      string `json:"date"`
	Sessions  int    `json:"sessions"`
	Secondary int    `json:"secondary"`
	ToolCalls int    `json:"tool_calls"`
}

// SourceHealth reports the availability of each data source plus non-fatal
// parse errors encountered during aggregation.
type SourceHealth struct {
	JsonlAvailable   bool     `json:"jsonl_available"`
	SqliteAvailable  bool     `json:"sqlite_available"`  // Codex only
	HistoryAvailable bool     `json:"history_available"` // Codex history.jsonl (or Claude history.jsonl)
	ProjectDir       string   `json:"project_dir"`       // resolved source directory (empty if unavailable)
	PartialErrors    []string `json:"partial_errors"`
}
