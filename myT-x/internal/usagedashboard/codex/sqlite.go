package codex

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteSummary is the aggregated view of Codex state SQLite filtered by cwd.
type SQLiteSummary struct {
	Available     bool
	TotalThreads  int
	ActiveDays    int
	TokensUsed    int64
	LastUpdatedAt time.Time
	Models        map[string]int // model name → thread count
}

func newSQLiteSummary(available bool) SQLiteSummary {
	return SQLiteSummary{
		Available: available,
		Models:    make(map[string]int),
	}
}

// ReadSQLite opens dbPath read-only and returns a cwd-scoped summary.
//
// Behavior:
//   - File does not exist → Available=false, error=nil (not a failure).
//   - File exists but schema differs (e.g., no threads table) → Available=true
//     with zeroed counters and a slog.Warn; no error returned (degraded result).
//   - Other errors (open/permission) are returned.
//
// Respects defensive-coding-checklist #155 (defer order after err check) and
// #179 (close + log warn on failure).
func ReadSQLite(dbPath, cwd string) (SQLiteSummary, error) {
	if strings.TrimSpace(dbPath) == "" {
		return newSQLiteSummary(false), nil
	}
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newSQLiteSummary(false), nil
		}
		return SQLiteSummary{}, fmt.Errorf("stat %s: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return newSQLiteSummary(true), fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("[usage-dashboard] close codex sqlite", "path", dbPath, "error", closeErr)
		}
	}()

	summary := newSQLiteSummary(true)

	// Query per-thread metadata. Tolerates missing table (degraded fallback).
	rows, err := db.Query(
		`SELECT created_at, updated_at, IFNULL(model, ''), tokens_used
		 FROM threads
		 WHERE cwd = ? COLLATE NOCASE AND archived = 0`,
		cwd,
	)
	if err != nil {
		if isMissingTableErr(err) {
			slog.Warn("[usage-dashboard] codex sqlite missing threads table", "path", dbPath)
			return summary, nil
		}
		return summary, fmt.Errorf("query threads: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("[usage-dashboard] close codex sqlite rows", "path", dbPath, "error", closeErr)
		}
	}()

	activeDays := make(map[string]struct{})
	var latest time.Time
	for rows.Next() {
		var createdAt, updatedAt int64
		var model string
		var tokens int64
		if err := rows.Scan(&createdAt, &updatedAt, &model, &tokens); err != nil {
			// Scan failure usually indicates schema drift (column type/count
			// mismatch). Log as warn so unexpected state is visible in diagnostics.
			slog.Warn("[usage-dashboard] codex sqlite scan row", "error", err)
			continue
		}
		summary.TotalThreads++
		summary.TokensUsed += tokens
		if model = strings.TrimSpace(model); model != "" {
			summary.Models[model]++
		}
		// created_at is recorded as unix seconds in state_5.sqlite.
		if createdAt > 0 {
			day := time.Unix(createdAt, 0).UTC().Format("2006-01-02")
			activeDays[day] = struct{}{}
		}
		if updatedAt > 0 {
			t := time.Unix(updatedAt, 0).UTC()
			if t.After(latest) {
				latest = t
			}
		}
	}
	if err := rows.Err(); err != nil {
		return newSQLiteSummary(true), fmt.Errorf("iterate threads: %w", err)
	}
	summary.ActiveDays = len(activeDays)
	summary.LastUpdatedAt = latest
	return summary, nil
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table")
}
