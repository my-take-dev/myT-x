package tmux

// SessionSnapshotDelta represents incremental updates for session snapshots.
type SessionSnapshotDelta struct {
	Upserts []SessionSnapshot `json:"upserts"`
	// Removed contains the names (not IDs) of sessions that were removed since the
	// previous snapshot. Frontend consumers should match these against session.name
	// fields rather than session.id.
	Removed []string `json:"removed"`
}
