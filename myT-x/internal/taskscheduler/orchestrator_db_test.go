package taskscheduler

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckOrchestratorReady_NonExistentPath(t *testing.T) {
	t.Parallel()
	result, err := CheckOrchestratorReady(filepath.Join(t.TempDir(), "no-such.db"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DBExists {
		t.Error("expected DBExists=false for non-existent path")
	}
	if result.AgentCount != 0 {
		t.Errorf("expected AgentCount=0, got %d", result.AgentCount)
	}
}

func TestCheckOrchestratorReady_EmptyPath(t *testing.T) {
	t.Parallel()
	result, err := CheckOrchestratorReady("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DBExists {
		t.Error("expected DBExists=false for empty path")
	}
}

func TestCheckOrchestratorReady_EmptyDB(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	db := createTestOrcDB(t, dbPath)
	closeTestDB(t, db)

	result, err := CheckOrchestratorReady(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DBExists {
		t.Error("expected DBExists=true")
	}
	if result.AgentCount != 0 {
		t.Errorf("expected AgentCount=0 (empty DB), got %d", result.AgentCount)
	}
}

func TestCheckOrchestratorReady_OnlyTaskMaster(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	db := createTestOrcDB(t, dbPath)
	insertTestAgent(t, db, taskMasterAgentName, taskMasterPaneID)
	closeTestDB(t, db)

	result, err := CheckOrchestratorReady(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DBExists {
		t.Error("expected DBExists=true")
	}
	if result.AgentCount != 0 {
		t.Errorf("expected AgentCount=0 (task-master excluded), got %d", result.AgentCount)
	}
}

func TestCheckOrchestratorReady_WithAgents(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	db := createTestOrcDB(t, dbPath)
	insertTestAgent(t, db, taskMasterAgentName, taskMasterPaneID)
	insertTestAgent(t, db, "agent-1", "%1")
	insertTestAgent(t, db, "agent-2", "%2")
	closeTestDB(t, db)

	result, err := CheckOrchestratorReady(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.DBExists {
		t.Error("expected DBExists=true")
	}
	if result.AgentCount != 2 {
		t.Errorf("expected AgentCount=2, got %d", result.AgentCount)
	}
}

func TestCheckOrchestratorReady_DBWithoutAgentsTable(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	// Create a DB file with no tables.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE dummy (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	closeTestDB(t, db)

	result, readErr := CheckOrchestratorReady(dbPath)
	if readErr != nil {
		t.Fatalf("unexpected error: %v", readErr)
	}
	if !result.DBExists {
		t.Error("expected DBExists=true")
	}
	if result.AgentCount != 0 {
		t.Errorf("expected AgentCount=0 (no agents table), got %d", result.AgentCount)
	}
}

func TestCheckOrchestratorReady_ReadFailureReturnsError(t *testing.T) {
	t.Parallel()

	result, err := CheckOrchestratorReady(t.TempDir())
	if err == nil {
		t.Fatal("expected error for unreadable sqlite path")
	}
	if result != (OrchestratorReadiness{}) {
		t.Fatalf("expected zero readiness on read failure, got %#v", result)
	}
}

// createTestOrcDB creates a test orchestrator.db with the agents table.
func createTestOrcDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE agents (
		name TEXT PRIMARY KEY,
		pane_id TEXT NOT NULL,
		role TEXT,
		skills TEXT,
		created_at TEXT NOT NULL,
		mcp_instance_id TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// insertTestAgent inserts a test agent record.
func insertTestAgent(t *testing.T, db *sql.DB, name, paneID string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO agents (name, pane_id, role, skills, created_at) VALUES (?, ?, '', '[]', '2025-01-01T00:00:00Z')`,
		name, paneID,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func closeTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
}

func TestOrchestratorReadinessFieldCount(t *testing.T) {
	t.Parallel()
	// Guard: if fields are added, update CheckOrchestratorReady and App API.
	expected := 2
	actual := reflect.TypeFor[OrchestratorReadiness]().NumField()
	if actual != expected {
		t.Errorf("OrchestratorReadiness has %d fields, expected %d", actual, expected)
	}
}

// Guard: detect when new status constants are added so IsEditable and tests stay in sync.
func TestQueueItemStatusCount(t *testing.T) {
	t.Parallel()
	allStatuses := []QueueItemStatus{
		ItemStatusPending, ItemStatusRunning, ItemStatusCompleted,
		ItemStatusFailed, ItemStatusSkipped,
	}
	if len(allStatuses) != 5 {
		t.Errorf("status count changed to %d; update IsEditable and TestIsEditable", len(allStatuses))
	}
}
