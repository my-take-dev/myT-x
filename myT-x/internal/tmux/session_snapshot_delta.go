package tmux

// SessionSnapshotDelta represents incremental updates for session snapshots.
type SessionSnapshotDelta struct {
	Upserts []SessionSnapshot `json:"upserts"`
	Removed []string          `json:"removed"`
}
