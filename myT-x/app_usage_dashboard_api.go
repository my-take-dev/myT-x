package main

import "myT-x/internal/usagedashboard"

// GetUsageDashboard returns usage statistics for the given session and mode.
//
// Parameters:
//   - sessionName: the tmux session whose effective work directory scopes the aggregation.
//   - mode: "claude", "codex", or "both" (any other value is treated as "both").
//   - force: when true, bypass the on-disk JSON cache and re-aggregate. The
//     "Refresh" button in the UI passes true; automatic loads pass false so a
//     fresh cached snapshot (within TTL) is reused without re-scanning JSONL files.
//
// Wails IPC contract: never returns nil slices inside the snapshot when error is nil;
// callers can safely iterate Skills/Agents/SlashCommands/DailyActivity without guards.
func (a *App) GetUsageDashboard(sessionName, mode string, force bool) (usagedashboard.UsageDashboardSnapshot, error) {
	return a.usageDashboard.GetUsageDashboard(sessionName, mode, force)
}
