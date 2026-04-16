package usagedashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// snapshotDir is the per-project subdirectory that holds myT-x metadata
	// (matches scheduler.templateDir convention).
	snapshotDir = ".myT-x"

	// snapshotFileName is the cached aggregation result file.
	snapshotFileName = "usage-dashboard.json"

	// snapshotSchemaVersion is the on-disk format version. Bump when
	// PersistedSnapshot fields change in a way that old readers cannot parse.
	snapshotSchemaVersion = 1

	// tempSnapshotPattern is the os.CreateTemp pattern for atomic write.
	tempSnapshotPattern = "usage-dashboard-*.json.tmp"
)

// DefaultSnapshotTTL controls how long a cached snapshot is considered
// fresh before automatic re-aggregation is triggered on next access.
const DefaultSnapshotTTL = 24 * time.Hour

// PersistedSnapshot is the on-disk JSON wrapper for the cached aggregation
// result. Both Claude and Codex are always populated regardless of the mode
// requested by callers; the service filters by mode at return time.
type PersistedSnapshot struct {
	SchemaVersion int               `json:"schema_version"`
	WorkDir       string            `json:"work_dir"`
	SavedAt       time.Time         `json:"saved_at"`
	Claude        *ClaudeUsageStats `json:"claude,omitempty"`
	Codex         *CodexUsageStats  `json:"codex,omitempty"`
}

// SnapshotRepository abstracts persistence of dashboard aggregation results
// so tests can substitute an in-memory implementation.
//
// Load contract:
//   - found=false, err=nil  → file absent (treated as cache miss).
//   - found=false, err!=nil → read or parse failure (caller logs + re-aggregates).
//   - found=true,  err=nil  → snapshot returned.
type SnapshotRepository interface {
	Load(workDir string) (PersistedSnapshot, bool, error)
	Save(snap PersistedSnapshot) error
}

// fileSnapshotRepository persists snapshots under
// <workDir>/.myT-x/usage-dashboard.json.
type fileSnapshotRepository struct{}

// NewFileSnapshotRepository returns the default file-backed repository.
func NewFileSnapshotRepository() SnapshotRepository {
	return &fileSnapshotRepository{}
}

// snapshotPath returns the on-disk JSON path for the given workDir.
func snapshotPath(workDir string) string {
	return filepath.Join(workDir, snapshotDir, snapshotFileName)
}

func schemaVersionMismatchMessage(foundVersion int) string {
	if foundVersion < snapshotSchemaVersion {
		return "[USAGE_DASHBOARD_DEBUG] snapshot schema version older than expected, ignoring cache"
	}
	return "[USAGE_DASHBOARD_DEBUG] snapshot schema version newer than expected, ignoring cache"
}

func (r *fileSnapshotRepository) Load(workDir string) (PersistedSnapshot, bool, error) {
	path := snapshotPath(workDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PersistedSnapshot{}, false, nil
		}
		return PersistedSnapshot{}, false, fmt.Errorf("read snapshot: %w", err)
	}
	var snap PersistedSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return PersistedSnapshot{}, false, fmt.Errorf("parse snapshot: %w", err)
	}
	if snap.SchemaVersion != snapshotSchemaVersion {
		slog.Warn(schemaVersionMismatchMessage(snap.SchemaVersion),
			"path", path, "version", snap.SchemaVersion, "expected", snapshotSchemaVersion)
		return PersistedSnapshot{}, false, nil
	}
	return snap, true, nil
}

func (r *fileSnapshotRepository) Save(snap PersistedSnapshot) error {
	if snap.WorkDir == "" {
		return errors.New("save snapshot: WorkDir is empty")
	}
	path := snapshotPath(snap.WorkDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	// Atomic write: write temp file in same dir, then rename
	// (#60 atomic write convention; matches scheduler.writeTemplates).
	tmp, err := os.CreateTemp(dir, tempSnapshotPattern)
	if err != nil {
		return fmt.Errorf("create temp snapshot: %w", err)
	}
	tmpPath := tmp.Name()

	// Single deferred cleanup keeps the close/remove ordering explicit:
	// on Windows the file must be closed before it can be removed or renamed.
	// Both flags are flipped only on the success path so any early return
	// safely closes the descriptor and drops the temp file.
	closed := false
	cleanup := true
	defer func() {
		if !closed {
			if closeErr := tmp.Close(); closeErr != nil {
				slog.Warn("[USAGE_DASHBOARD_DEBUG] close temp snapshot during cleanup failed",
					"path", tmpPath, "err", closeErr)
			}
		}
		if cleanup {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				slog.Warn("[USAGE_DASHBOARD_DEBUG] remove temp snapshot during cleanup failed",
					"path", tmpPath, "err", removeErr)
			}
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp snapshot: %w", err)
	}
	closed = true
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp snapshot: %w", err)
	}
	cleanup = false
	return nil
}

// isExpired reports whether savedAt is older than ttl relative to now.
// A zero savedAt is always treated as expired so a corrupt cache cannot
// be served (#157 API contract: never return an invalid LastUpdatedAt).
// ttl <= 0 falls back to DefaultSnapshotTTL.
func isExpired(savedAt time.Time, now time.Time, ttl time.Duration) bool {
	if savedAt.IsZero() {
		return true
	}
	if ttl <= 0 {
		ttl = DefaultSnapshotTTL
	}
	return now.Sub(savedAt) >= ttl
}
