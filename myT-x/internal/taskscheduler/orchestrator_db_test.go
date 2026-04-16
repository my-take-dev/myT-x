package taskscheduler

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	orcdomain "myT-x/internal/mcp/agent-orchestrator/domain"
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

func TestAbandonPendingTask_IgnoresAlreadyTransitionedTask(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	prepareTaskSchedulerDB(t, dbPath)

	db, err := openOrchestratorDB(dbPath)
	if err != nil {
		t.Fatalf("openOrchestratorDB: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	sentAt := time.Now().UTC().Format(time.RFC3339)
	completedAt := time.Now().UTC().Add(time.Second).Format(time.RFC3339)
	if _, err := db.db.Exec(
		`INSERT INTO tasks (
			task_id, agent_name, assignee_pane_id, sender_pane_id,
			sender_name, sender_instance_id, send_message_id, status, sent_at, completed_at, is_now_session
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", taskMasterAgentName, "%1", taskMasterPaneID,
		taskMasterAgentName, "", "msg-1", string(orcdomain.TaskStatusCompleted), sentAt, completedAt, 1,
	); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	if err := db.abandonPendingTask("task-1"); err != nil {
		t.Fatalf("abandonPendingTask should ignore already transitioned tasks: %v", err)
	}

	var status string
	if err := db.db.QueryRow(`SELECT status FROM tasks WHERE task_id = ?`, "task-1").Scan(&status); err != nil {
		t.Fatalf("query task status: %v", err)
	}
	if status != string(orcdomain.TaskStatusCompleted) {
		t.Fatalf("task status after ignored abandon = %q, want %q", status, orcdomain.TaskStatusCompleted)
	}
}

func TestAbandonPendingTask_ReturnsErrorForMissingTask(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "orchestrator.db")
	prepareTaskSchedulerDB(t, dbPath)

	db, err := openOrchestratorDB(dbPath)
	if err != nil {
		t.Fatalf("openOrchestratorDB: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	if err := db.abandonPendingTask("missing-task"); err == nil {
		t.Fatal("abandonPendingTask should fail for a missing task")
	}
}
