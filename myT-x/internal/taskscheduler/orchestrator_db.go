package taskscheduler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// OrchestratorReadiness holds the result of an orchestrator readiness check.
type OrchestratorReadiness struct {
	DBExists   bool
	AgentCount int
}

// CheckOrchestratorReady checks if orchestrator.db exists and has registered agents.
// task-master is the scheduler's own virtual agent and is excluded from the count.
func CheckOrchestratorReady(dbPath string) (OrchestratorReadiness, error) {
	if dbPath == "" {
		return OrchestratorReadiness{}, nil
	}

	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return OrchestratorReadiness{DBExists: false, AgentCount: 0}, nil
		}
		slog.Warn("[DEBUG-TASK-SCHEDULER] readiness: stat db path", "path", dbPath, "error", err)
		return OrchestratorReadiness{DBExists: false, AgentCount: 0},
			fmt.Errorf("stat orchestrator db: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return OrchestratorReadiness{DBExists: true, AgentCount: 0},
			fmt.Errorf("open orchestrator db for readiness check: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("[DEBUG-TASK-SCHEDULER] close readiness db", "error", closeErr)
		}
	}()

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM agents WHERE name != ?`, taskMasterAgentName).Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "no such table: agents") {
			// The DB file exists but orchestrator initialization has not created the agents table yet.
			slog.Warn("[DEBUG-TASK-SCHEDULER] readiness agent count query failed", "error", err)
			return OrchestratorReadiness{DBExists: true, AgentCount: 0}, nil
		}
		slog.Warn("[DEBUG-TASK-SCHEDULER] readiness query failed", "path", dbPath, "error", err)
		return OrchestratorReadiness{}, fmt.Errorf("query orchestrator readiness: %w", err)
	}

	return OrchestratorReadiness{DBExists: true, AgentCount: count}, nil
}

// orchestratorDB wraps a direct connection to orchestrator.db for
// task-master agent registration and task tracking operations.
// This is a standalone connection separate from the MCP orchestrator
// runtime to avoid coupling the task scheduler to the MCP lifecycle.
type orchestratorDB struct {
	db *sql.DB
}

// openOrchestratorDB opens a connection to orchestrator.db in WAL mode.
func openOrchestratorDB(dbPath string) (*orchestratorDB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("orchestrator db path is empty")
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open orchestrator db: %w", err)
	}
	// Verify connectivity.
	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("[DEBUG-TASK-SCHEDULER] close orchestrator db after ping failure", "error", closeErr)
		}
		return nil, fmt.Errorf("ping orchestrator db: %w", err)
	}
	return &orchestratorDB{db: db}, nil
}

// Close closes the database connection.
func (o *orchestratorDB) Close() error {
	if o.db == nil {
		return nil
	}
	return o.db.Close()
}

const (
	taskMasterAgentName = "task-master"
	taskMasterPaneID    = "%virtual-task-master"
	taskMasterRole      = "Task Scheduler"
)

// ensureTaskMasterAgent registers or updates the task-master virtual agent.
func (o *orchestratorDB) ensureTaskMasterAgent() error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := o.db.Exec(
		`INSERT INTO agents (name, pane_id, role, skills, created_at, mcp_instance_id)
		 VALUES (?, ?, ?, '[]', ?, '')
		 ON CONFLICT(name) DO UPDATE SET pane_id = excluded.pane_id`,
		taskMasterAgentName, taskMasterPaneID, taskMasterRole, now,
	)
	if err != nil {
		return fmt.Errorf("upsert task-master agent: %w", err)
	}
	return nil
}

// createTask creates a task record in the orchestrator tasks table.
// Returns the generated task_id and send_message_id.
func (o *orchestratorDB) createTask(assigneePaneID, message string) (taskID, msgID string, err error) {
	taskID, err = generateHexID("t-")
	if err != nil {
		return "", "", fmt.Errorf("generate task id: %w", err)
	}
	msgID, err = generateHexID("m-")
	if err != nil {
		return "", "", fmt.Errorf("generate message id: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := o.db.Begin()
	if err != nil {
		return "", "", fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				slog.Warn("[DEBUG-TASK-SCHEDULER] rollback failed", "error", rbErr)
			}
		}
	}()

	// Save the message content.
	_, err = tx.Exec(
		`INSERT INTO send_messages (id, content, created_at) VALUES (?, ?, ?)`,
		msgID, message, now,
	)
	if err != nil {
		return "", "", fmt.Errorf("insert send_message: %w", err)
	}

	// Create the task.
	_, err = tx.Exec(
		`INSERT INTO tasks (task_id, agent_name, assignee_pane_id, sender_pane_id,
			sender_name, sender_instance_id, send_message_id, status, sent_at, is_now_session)
		 VALUES (?, ?, ?, ?, ?, '', ?, 'pending', ?, 1)`,
		taskID, taskMasterAgentName, assigneePaneID, taskMasterPaneID,
		taskMasterAgentName, msgID, now,
	)
	if err != nil {
		return "", "", fmt.Errorf("insert task: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit tx: %w", err)
	}
	return taskID, msgID, nil
}

// pollTaskStatus checks the current status of a task.
// Returns the status string and completed_at timestamp.
func (o *orchestratorDB) pollTaskStatus(taskID string) (status string, completedAt string, err error) {
	if taskID == "" {
		return "", "", fmt.Errorf("task id is empty")
	}
	err = o.db.QueryRow(
		`SELECT status, COALESCE(completed_at, '') FROM tasks WHERE task_id = ?`,
		taskID,
	).Scan(&status, &completedAt)
	if err != nil {
		return "", "", fmt.Errorf("query task %s: %w", taskID, err)
	}
	return status, completedAt, nil
}

// generateHexID generates a random hex-encoded ID with the given prefix.
// Output format: prefix + hex(6 bytes) = prefix + 12 hex characters.
func generateHexID(prefix string) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}
