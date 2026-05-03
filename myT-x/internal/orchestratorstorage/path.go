package orchestratorstorage

import "myT-x/internal/sessioninfo"

const dbFileName = "orchestrator.db"

// DBPath returns the session-info path for the orchestrator SQLite database.
// Per knowledge/session-scoped-storage-policy.md, legacy project .myT-x DB
// paths are not migrated or used as fallback storage.
func DBPath(configDir, workDir string) (string, error) {
	return sessioninfo.FilePath(configDir, workDir, dbFileName)
}
