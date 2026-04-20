package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestMigrateRemovesLegacyTasksAgentForeignKey(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
CREATE TABLE agents (
	name TEXT PRIMARY KEY,
	pane_id TEXT NOT NULL,
	role TEXT,
	skills TEXT,
	created_at TEXT,
	mcp_instance_id TEXT
);
CREATE TABLE tasks (
	task_id TEXT PRIMARY KEY,
	agent_name TEXT NOT NULL REFERENCES agents(name),
	assignee_pane_id TEXT,
	sender_pane_id TEXT,
	sender_name TEXT,
	sender_instance_id TEXT,
	send_message_id TEXT,
	send_response_id TEXT,
	status TEXT NOT NULL DEFAULT 'pending',
	sent_at TEXT NOT NULL DEFAULT (datetime('now')),
	completed_at TEXT,
	acknowledged_at TEXT,
	cancelled_at TEXT,
	cancel_reason TEXT,
	progress_pct INTEGER,
	progress_note TEXT,
	progress_updated_at TEXT,
	expires_at TEXT,
	group_id TEXT,
	is_now_session INTEGER NOT NULL DEFAULT 0
);
INSERT INTO agents(name, pane_id) VALUES ('worker', '%1');
INSERT INTO tasks(task_id, agent_name, status, sent_at, is_now_session) VALUES ('t-legacy', 'worker', 'pending', '2026-04-02T10:00:00Z', 1);
`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	hasLegacyForeignKey, err := tasksTableHasLegacyAgentForeignKey(context.Background(), conn)
	if closeErr := conn.Close(); closeErr != nil {
		t.Fatalf("close pre-migration conn: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("tasksTableHasLegacyAgentForeignKey(before migrate): %v", err)
	}
	if !hasLegacyForeignKey {
		t.Fatal("tasksTableHasLegacyAgentForeignKey(before migrate) = false, want true")
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	conn, err = st.db.Conn(context.Background())
	if err != nil {
		t.Fatalf("st.db.Conn: %v", err)
	}
	hasLegacyForeignKey, err = tasksTableHasLegacyAgentForeignKey(context.Background(), conn)
	if closeErr := conn.Close(); closeErr != nil {
		t.Fatalf("close migrated conn: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("tasksTableHasLegacyAgentForeignKey(after migrate): %v", err)
	}
	if hasLegacyForeignKey {
		t.Fatal("tasksTableHasLegacyAgentForeignKey(after migrate) = true, want false")
	}

	task, err := st.GetTask(context.Background(), "t-legacy")
	if err != nil {
		t.Fatalf("GetTask(t-legacy): %v", err)
	}
	if task.AgentName != "worker" || task.Status != domain.TaskStatusPending {
		t.Fatalf("migrated task = %+v", task)
	}
}

func TestMigrateAddsStorageModeChecksToLegacyPayloadTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-payload.db")

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`
CREATE TABLE send_messages (
	id TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	storage_mode TEXT NOT NULL DEFAULT 'inline',
	content_preview TEXT NOT NULL DEFAULT '',
	artifact_paths_json TEXT NOT NULL DEFAULT '[]',
	part_count INTEGER NOT NULL DEFAULT 0,
	content_chars INTEGER NOT NULL DEFAULT 0,
	sha256 TEXT NOT NULL DEFAULT ''
);
CREATE TABLE send_responses (
	id TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	storage_mode TEXT NOT NULL DEFAULT 'inline',
	content_preview TEXT NOT NULL DEFAULT '',
	artifact_paths_json TEXT NOT NULL DEFAULT '[]',
	part_count INTEGER NOT NULL DEFAULT 0,
	content_chars INTEGER NOT NULL DEFAULT 0,
	sha256 TEXT NOT NULL DEFAULT ''
);
INSERT INTO send_messages(id, content, created_at, storage_mode) VALUES ('m-legacy', '', '2026-04-18T01:02:03Z', 'file');
INSERT INTO send_responses(id, content, created_at, storage_mode) VALUES ('r-legacy', '', '2026-04-18T01:02:03Z', 'multipart_file');
`); err != nil {
		t.Fatalf("seed legacy payload schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	message, err := st.GetMessage(context.Background(), "m-legacy")
	if err != nil {
		t.Fatalf("GetMessage(m-legacy): %v", err)
	}
	if message.StorageMode != domain.MessageStorageFile {
		t.Fatalf("migrated message storage_mode = %q, want file", message.StorageMode)
	}
	response, err := st.GetResponse(context.Background(), "r-legacy")
	if err != nil {
		t.Fatalf("GetResponse(r-legacy): %v", err)
	}
	if response.StorageMode != domain.MessageStorageMultipartFile {
		t.Fatalf("migrated response storage_mode = %q, want multipart_file", response.StorageMode)
	}

	if _, err := st.db.ExecContext(context.Background(), `INSERT INTO send_messages (
		id, content, created_at, storage_mode, content_preview, artifact_paths_json, part_count, content_chars, sha256
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"m-invalid-after-migrate", "", "2026-04-18T01:02:03Z", "zipfile", "", "[]", 0, 0, "",
	); err == nil {
		t.Fatal("insert invalid migrated message row = nil error, want CHECK constraint failure")
	}
	if _, err := st.db.ExecContext(context.Background(), `INSERT INTO send_responses (
		id, content, created_at, storage_mode, content_preview, artifact_paths_json, part_count, content_chars, sha256
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"r-invalid-after-migrate", "", "2026-04-18T01:02:03Z", "zipfile", "", "[]", 0, 0, "",
	); err == nil {
		t.Fatal("insert invalid migrated response row = nil error, want CHECK constraint failure")
	}
}

func TestUpsertAndGetAgent(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	agent := domain.Agent{
		Name:   "codex",
		PaneID: "%1",
		Role:   "バックエンド実装",
		Skills: []domain.Skill{{Name: "Go"}, {Name: "API設計"}},
	}
	if err := st.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	got, err := st.GetAgent(ctx, "codex")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Name != "codex" || got.PaneID != "%1" || got.Role != "バックエンド実装" {
		t.Errorf("got %+v", got)
	}
	if len(got.Skills) != 2 || got.Skills[0].Name != "Go" || got.Skills[1].Name != "API設計" {
		t.Errorf("skills = %v", got.Skills)
	}
}

func TestGetAgentByPaneID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	got, err := st.GetAgentByPaneID(ctx, "%1")
	if err != nil {
		t.Fatalf("GetAgentByPaneID: %v", err)
	}
	if got.Name != "codex" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetAgentByPaneIDRejectsDuplicatePaneRegistration(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "b", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	_, err := st.GetAgentByPaneID(ctx, "%1")
	if err == nil || err.Error() != `multiple agents registered for pane_id "%1"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAgentByMCPInstanceID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1", MCPInstanceID: "mcp-1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	got, err := st.GetAgentByMCPInstanceID(ctx, "mcp-1")
	if err != nil {
		t.Fatalf("GetAgentByMCPInstanceID: %v", err)
	}
	if got.Name != "codex" || got.MCPInstanceID != "mcp-1" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetAgentByMCPInstanceIDReturnsNotFound(t *testing.T) {
	st := newTestStore(t)

	_, err := st.GetAgentByMCPInstanceID(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestGetAgentByMCPInstanceIDRejectsDuplicateRegistration(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1", MCPInstanceID: "mcp-1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "b", PaneID: "%2", MCPInstanceID: "mcp-1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	_, err := st.GetAgentByMCPInstanceID(ctx, "mcp-1")
	if err == nil || err.Error() != `multiple agents registered for mcp_instance_id "mcp-1"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertAgentOverwrite(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1", Role: "old"})
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%2", Role: "new"})

	got, err := st.GetAgent(ctx, "codex")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.PaneID != "%2" || got.Role != "new" {
		t.Errorf("overwrite failed: got %+v", got)
	}
}

func TestUpsertAgentOverwriteUpdatesMCPInstanceID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1", MCPInstanceID: "mcp-1"}); err != nil {
		t.Fatalf("UpsertAgent first: %v", err)
	}
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1", MCPInstanceID: "mcp-2"}); err != nil {
		t.Fatalf("UpsertAgent second: %v", err)
	}

	got, err := st.GetAgent(ctx, "codex")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.MCPInstanceID != "mcp-2" {
		t.Fatalf("mcp instance overwrite failed: got %+v", got)
	}
}

func TestGetAgentNotFound(t *testing.T) {
	st := newTestStore(t)
	_, err := st.GetAgent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

func TestListAgents(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "b-agent", PaneID: "%2"})
	st.UpsertAgent(ctx, domain.Agent{Name: "a-agent", PaneID: "%1"})

	agents, err := st.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("got %d agents, want 2", len(agents))
	}
	if agents[0].Name != "a-agent" {
		t.Errorf("expected sorted, got %s first", agents[0].Name)
	}
}

func TestDeleteAgentsByPaneID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "b", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "c", PaneID: "%2"})
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "a", Status: domain.AgentWorkStatusBusy, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(a): %v", err)
	}
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "c", Status: domain.AgentWorkStatusIdle, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(c): %v", err)
	}

	if err := st.DeleteAgentsByPaneID(ctx, "%1"); err != nil {
		t.Fatalf("DeleteAgentsByPaneID: %v", err)
	}

	agents, _ := st.ListAgents(ctx)
	if len(agents) != 1 || agents[0].Name != "c" {
		t.Errorf("got %+v", agents)
	}

	statuses, err := st.ListAgentStatuses(ctx)
	if err != nil {
		t.Fatalf("ListAgentStatuses: %v", err)
	}
	if len(statuses) != 1 || statuses[0].AgentName != "c" {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestReplaceAgentRegistrationPreservesExistingStatus(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1", Role: "existing"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "old", PaneID: "%2"}); err != nil {
		t.Fatalf("UpsertAgent(old): %v", err)
	}
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{
		AgentName:     "worker",
		Status:        domain.AgentWorkStatusWorking,
		CurrentTaskID: "t-123",
		Note:          "running",
		UpdatedAt:     "2026-04-14T10:00:00Z",
	}); err != nil {
		t.Fatalf("UpsertAgentStatus(worker): %v", err)
	}

	defaultStatus := &domain.AgentStatus{
		AgentName: "worker",
		Status:    domain.AgentWorkStatusIdle,
		UpdatedAt: "2026-04-15T00:00:00Z",
	}
	if err := st.ReplaceAgentRegistration(ctx, domain.Agent{Name: "worker", PaneID: "%2", Role: "reviewer"}, defaultStatus); err != nil {
		t.Fatalf("ReplaceAgentRegistration: %v", err)
	}

	agent, err := st.GetAgent(ctx, "worker")
	if err != nil {
		t.Fatalf("GetAgent(worker): %v", err)
	}
	if agent.PaneID != "%2" || agent.Role != "reviewer" {
		t.Fatalf("worker agent = %+v", agent)
	}
	if _, err := st.GetAgent(ctx, "old"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old agent should be removed, got err=%v", err)
	}
	status, err := st.GetAgentStatus(ctx, "worker")
	if err != nil {
		t.Fatalf("GetAgentStatus(worker): %v", err)
	}
	if status.Status != domain.AgentWorkStatusWorking || status.CurrentTaskID != "t-123" || status.Note != "running" {
		t.Fatalf("status = %+v", status)
	}
}

func TestCreateAndGetTask(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"})

	task := domain.Task{
		ID:             "t-001",
		AgentName:      "codex",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
		SentAt:         "2026-03-07T10:00:00Z",
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := st.GetTask(ctx, "t-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ID != "t-001" || got.Status != "pending" || got.AgentName != "codex" || got.AssigneePaneID != "%1" || got.SenderName != "orchestrator" {
		t.Errorf("got %+v", got)
	}
}

func TestCreateTaskWithSendMessageID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"})

	// メッセージを先に保存
	if err := st.SaveMessage(ctx, domain.TaskMessage{ID: "m-001", Content: "hello", CreatedAt: "2026-03-07T10:00:00Z"}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	task := domain.Task{
		ID:            "t-001",
		AgentName:     "codex",
		SendMessageID: "m-001",
		Status:        "pending",
		SentAt:        "2026-03-07T10:00:00Z",
	}
	if err := st.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := st.GetTask(ctx, "t-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.SendMessageID != "m-001" {
		t.Errorf("send_message_id = %q, want m-001", got.SendMessageID)
	}
}

func TestListTasksWithFilter(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "b", PaneID: "%2"})

	st.CreateTask(ctx, domain.Task{ID: "t-1", AgentName: "a", Status: "pending", SentAt: "2026-03-07T10:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-2", AgentName: "a", Status: "completed", SentAt: "2026-03-07T11:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-3", AgentName: "b", Status: "pending", SentAt: "2026-03-07T12:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-4", AgentName: "a", Status: "failed", SentAt: "2026-03-07T13:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-5", AgentName: "b", Status: "abandoned", SentAt: "2026-03-07T14:00:00Z"})

	tests := []struct {
		name   string
		filter domain.TaskFilter
		count  int
	}{
		{"all", domain.TaskFilter{Status: domain.TaskStatusFilterAll}, 5},
		{"pending only", domain.TaskFilter{Status: "pending"}, 2},
		{"completed only", domain.TaskFilter{Status: "completed"}, 1},
		{"failed only", domain.TaskFilter{Status: "failed"}, 1},
		{"abandoned only", domain.TaskFilter{Status: "abandoned"}, 1},
		{"agent a pending", domain.TaskFilter{Status: "pending", AgentName: "a"}, 1},
		{"agent b all", domain.TaskFilter{Status: domain.TaskStatusFilterAll, AgentName: "b"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := st.ListTasks(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(tasks) != tt.count {
				t.Errorf("got %d tasks, want %d", len(tasks), tt.count)
			}
		})
	}
}

func TestCompleteTask(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"})
	st.CreateTask(ctx, domain.Task{ID: "t-001", AgentName: "codex", Status: "pending", SentAt: "2026-03-07T10:00:00Z"})

	// レスポンスを保存
	if err := st.SaveResponse(ctx, domain.TaskMessage{ID: "r-001", Content: "done", CreatedAt: "2026-03-08T12:34:56Z"}); err != nil {
		t.Fatalf("SaveResponse: %v", err)
	}

	completedAt := "2026-03-08T12:34:56Z"
	if err := st.CompleteTask(ctx, "t-001", "r-001", completedAt); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	got, _ := st.GetTask(ctx, "t-001")
	if got.Status != "completed" || got.SendResponseID != "r-001" || got.CompletedAt != completedAt {
		t.Errorf("got %+v", got)
	}
}

func TestCompleteTaskNotFound(t *testing.T) {
	st := newTestStore(t)
	err := st.CompleteTask(context.Background(), "nonexistent", "", "2026-03-08T12:34:56Z")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestCompleteTaskRejectsNonPending(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"})
	st.CreateTask(ctx, domain.Task{ID: "t-001", AgentName: "codex", Status: "completed", SentAt: "2026-03-07T10:00:00Z"})

	err := st.CompleteTask(ctx, "t-001", "", "2026-03-08T12:34:56Z")
	if err == nil {
		t.Fatal("expected error for non-pending task")
	}
}

func TestMarkTaskFailed(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{ID: "t-001", AgentName: "codex", Status: "pending", SentAt: "2026-03-07T10:00:00Z"}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := st.MarkTaskFailed(ctx, "t-001"); err != nil {
		t.Fatalf("MarkTaskFailed: %v", err)
	}

	got, err := st.GetTask(ctx, "t-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "failed" || got.CompletedAt == "" {
		t.Fatalf("got %+v", got)
	}
}

func TestAbandonTasksByPaneID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "b", PaneID: "%2"})
	st.CreateTask(ctx, domain.Task{ID: "t-1", AgentName: "a", AssigneePaneID: "%1", Status: "pending", SentAt: "2026-03-07T10:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-1b", AgentName: "a", AssigneePaneID: "%1", Status: "blocked", SentAt: "2026-03-07T10:30:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-2", AgentName: "a", Status: "completed", SentAt: "2026-03-07T11:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-3", AgentName: "b", Status: "pending", SentAt: "2026-03-07T12:00:00Z"})

	if err := st.AbandonTasksByPaneID(ctx, "%1"); err != nil {
		t.Fatalf("AbandonTasksByPaneID: %v", err)
	}

	t1, _ := st.GetTask(ctx, "t-1")
	t2, _ := st.GetTask(ctx, "t-2")
	t3, _ := st.GetTask(ctx, "t-3")

	if t1.Status != "abandoned" {
		t.Errorf("t-1 status = %s, want abandoned", t1.Status)
	}
	t1b, _ := st.GetTask(ctx, "t-1b")
	if t1b.Status != "abandoned" {
		t.Errorf("t-1b status = %s, want abandoned", t1b.Status)
	}
	if t2.Status != "completed" {
		t.Errorf("t-2 status = %s, want completed (should not change)", t2.Status)
	}
	if t3.Status != "pending" {
		t.Errorf("t-3 status = %s, want pending (different pane)", t3.Status)
	}
}

func TestAbandonTasksByPaneIDUsesAssigneePaneSnapshot(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"})
	if err := st.CreateTask(ctx, domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         "pending",
		SentAt:         "2026-03-07T10:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := st.DeleteAgentsByPaneID(ctx, "%1"); err != nil {
		t.Fatalf("DeleteAgentsByPaneID: %v", err)
	}
	if err := st.UpsertAgent(ctx, domain.Agent{Name: "renamed", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	if err := st.AbandonTasksByPaneID(ctx, "%1"); err != nil {
		t.Fatalf("AbandonTasksByPaneID: %v", err)
	}

	got, err := st.GetTask(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "abandoned" {
		t.Fatalf("t-1 status = %s, want abandoned", got.Status)
	}
}

func TestSaveAndRetrieveMessage(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	msg := domain.TaskMessage{ID: "m-001", Content: "hello world", CreatedAt: "2026-03-07T10:00:00Z"}
	if err := st.SaveMessage(ctx, msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	// メッセージが保存されたことを確認（直接SQLで確認）
	var content string
	err := st.db.QueryRowContext(ctx, `SELECT content FROM send_messages WHERE id = ?`, "m-001").Scan(&content)
	if err != nil {
		t.Fatalf("query message: %v", err)
	}
	if content != "hello world" {
		t.Fatalf("content = %q, want 'hello world'", content)
	}
}

func TestSaveAndGetMessagePreservesStoredPayloadMetadata(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	msg := domain.TaskMessage{
		ID:             "m-meta",
		Content:        "",
		CreatedAt:      "2026-03-07T10:00:00Z",
		StorageMode:    domain.MessageStorageMultipartFile,
		ContentPreview: "stored preview",
		ArtifactPaths: []string{
			".myT-x/orchestrator/payloads/task-1/message.manifest.json",
			".myT-x/orchestrator/payloads/task-1/message.part1.txt",
		},
		PartCount:    2,
		ContentChars: 32001,
		SHA256:       "abc123",
	}
	if err := st.SaveMessage(ctx, msg); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	got, err := st.GetMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}

	if !reflect.DeepEqual(got, normalizeTaskMessage(msg)) {
		t.Fatalf("GetMessage() = %+v, want %+v", got, normalizeTaskMessage(msg))
	}
}

func TestSaveAndRetrieveResponse(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	resp := domain.TaskMessage{ID: "r-001", Content: "task completed", CreatedAt: "2026-03-07T10:00:00Z"}
	if err := st.SaveResponse(ctx, resp); err != nil {
		t.Fatalf("SaveResponse: %v", err)
	}

	// レスポンスが保存されたことを確認（直接SQLで確認）
	var content string
	err := st.db.QueryRowContext(ctx, `SELECT content FROM send_responses WHERE id = ?`, "r-001").Scan(&content)
	if err != nil {
		t.Fatalf("query response: %v", err)
	}
	if content != "task completed" {
		t.Fatalf("content = %q, want 'task completed'", content)
	}
}

func TestSaveAndGetResponsePreservesStoredPayloadMetadata(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	resp := domain.TaskMessage{
		ID:             "r-meta",
		Content:        "",
		CreatedAt:      "2026-03-07T10:00:00Z",
		StorageMode:    domain.MessageStorageFile,
		ContentPreview: "stored response preview",
		ArtifactPaths:  []string{".myT-x/orchestrator/payloads/task-1/response.txt"},
		PartCount:      1,
		ContentChars:   16001,
		SHA256:         "def456",
	}
	if err := st.SaveResponse(ctx, resp); err != nil {
		t.Fatalf("SaveResponse: %v", err)
	}

	got, err := st.GetResponse(ctx, resp.ID)
	if err != nil {
		t.Fatalf("GetResponse: %v", err)
	}

	if !reflect.DeepEqual(got, normalizeTaskMessage(resp)) {
		t.Fatalf("GetResponse() = %+v, want %+v", got, normalizeTaskMessage(resp))
	}
}

func TestSaveMessageRejectsInvalidStorageMode(t *testing.T) {
	st := newTestStore(t)
	err := st.SaveMessage(context.Background(), domain.TaskMessage{
		ID:          "m-invalid",
		Content:     "",
		CreatedAt:   "2026-04-18T01:02:03Z",
		StorageMode: domain.MessageStorageMode("zipfile"),
	})
	if err == nil || !strings.Contains(err.Error(), `invalid storage_mode "zipfile"`) {
		t.Fatalf("SaveMessage() error = %v, want invalid storage_mode", err)
	}
}

func TestSendResponsesTableRejectsInvalidStorageMode(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	_, err := st.db.ExecContext(ctx, `INSERT INTO send_responses (
		id, content, created_at, storage_mode, content_preview, artifact_paths_json, part_count, content_chars, sha256
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"r-invalid", "", "2026-04-18T01:02:03Z", "zipfile", "", "[]", 0, 0, "",
	)
	if err == nil {
		t.Fatal("insert invalid response row = nil error, want CHECK constraint failure")
	}
}

func TestConfigGetSet(t *testing.T) {
	st := newTestStore(t)

	if err := st.SetConfig("key1", "value1"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	got, err := st.GetConfig("key1")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if got != "value1" {
		t.Errorf("got %q, want %q", got, "value1")
	}

	st.SetConfig("key1", "value2")
	got, _ = st.GetConfig("key1")
	if got != "value2" {
		t.Errorf("overwrite: got %q, want %q", got, "value2")
	}
}

func TestNewCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "deep", "test.db")

	st, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer st.Close()

	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	st := newTestStore(t)

	if err := st.Migrate(); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

type discardMigrationConnStub struct {
	called bool
}

func (s *discardMigrationConnStub) Raw(fn func(any) error) error {
	s.called = true
	return fn(nil)
}

func TestDiscardMigrationConnMarksConnectionBad(t *testing.T) {
	stub := &discardMigrationConnStub{}

	if err := discardMigrationConn(stub); err != nil {
		t.Fatalf("discardMigrationConn(stub) error = %v", err)
	}
	if !stub.called {
		t.Fatal("discardMigrationConn should call Raw")
	}
}

type discardMigrationConnErrorStub struct{}

func (discardMigrationConnErrorStub) Raw(func(any) error) error {
	return driver.ErrBadConn
}

func TestDiscardMigrationConnTreatsDriverErrBadConnAsSuccess(t *testing.T) {
	if err := discardMigrationConn(discardMigrationConnErrorStub{}); err != nil {
		t.Fatalf("discardMigrationConn(driver.ErrBadConn) error = %v", err)
	}
}

type restoreMigrationConnStub struct {
	execErr     error
	closeErr    error
	rawErr      error
	execCalled  bool
	closeCalled bool
	rawCalled   bool
}

func (s *restoreMigrationConnStub) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	s.execCalled = true
	return nil, s.execErr
}

func (s *restoreMigrationConnStub) Close() error {
	s.closeCalled = true
	return s.closeErr
}

func (s *restoreMigrationConnStub) Raw(fn func(any) error) error {
	s.rawCalled = true
	if s.rawErr != nil {
		return s.rawErr
	}
	return fn(nil)
}

func TestRestoreForeignKeysAndReleaseConnDiscardsConnectionWhenPragmaRestoreFails(t *testing.T) {
	stub := &restoreMigrationConnStub{execErr: errors.New("pragma restore failed")}

	var retErr error
	restoreForeignKeysAndReleaseConn(context.Background(), stub, &retErr)

	if !stub.execCalled {
		t.Fatal("restoreForeignKeysAndReleaseConn should restore the pragma")
	}
	if !stub.rawCalled {
		t.Fatal("restoreForeignKeysAndReleaseConn should discard the connection after restore failure")
	}
	if !stub.closeCalled {
		t.Fatal("restoreForeignKeysAndReleaseConn should always close the connection")
	}
	if retErr == nil || !strings.Contains(retErr.Error(), "re-enable foreign keys") {
		t.Fatalf("retErr = %v, want re-enable foreign keys error", retErr)
	}
}

func TestRestoreForeignKeysAndReleaseConnPreservesPrimaryErrorWhenDiscardFails(t *testing.T) {
	stub := &restoreMigrationConnStub{
		execErr: errors.New("pragma restore failed"),
		rawErr:  errors.New("discard failed"),
	}

	primaryErr := errors.New("primary failure")
	restoreForeignKeysAndReleaseConn(context.Background(), stub, &primaryErr)

	if !stub.rawCalled {
		t.Fatal("restoreForeignKeysAndReleaseConn should try to discard the broken connection")
	}
	if primaryErr.Error() != "primary failure" {
		t.Fatalf("primaryErr = %v, want original error preserved", primaryErr)
	}
}

func TestUnmarshalSkillsJSONFormats(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantFirst domain.Skill
		wantErr   bool
	}{
		{
			name:      "new format with description",
			input:     `[{"name":"Go","description":"backend dev"}]`,
			wantLen:   1,
			wantFirst: domain.Skill{Name: "Go", Description: "backend dev"},
		},
		{
			name:      "new format without description",
			input:     `[{"name":"Go"}]`,
			wantLen:   1,
			wantFirst: domain.Skill{Name: "Go"},
		},
		{
			name:      "legacy string format",
			input:     `["Go","API設計"]`,
			wantLen:   2,
			wantFirst: domain.Skill{Name: "Go"},
		},
		{
			name:    "invalid JSON",
			input:   `not-json`,
			wantErr: true,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalSkillsJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unmarshalSkillsJSON(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d skills, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && got[0] != tt.wantFirst {
				t.Fatalf("first skill = %+v, want %+v", got[0], tt.wantFirst)
			}
		})
	}
}

func TestGetAgentReadLegacySkillsFormat(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// 旧形式で直接DBに挿入
	_, err := st.db.ExecContext(ctx,
		`INSERT INTO agents (name, pane_id, role, skills, created_at) VALUES (?, ?, ?, ?, ?)`,
		"legacy-agent", "%5", "worker", `["Go","testing"]`, "2026-03-14T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert legacy: %v", err)
	}

	got, err := st.GetAgent(ctx, "legacy-agent")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if len(got.Skills) != 2 || got.Skills[0].Name != "Go" || got.Skills[1].Name != "testing" {
		t.Fatalf("skills = %+v, want [{Go} {testing}]", got.Skills)
	}
	if got.Skills[0].Description != "" {
		t.Fatalf("legacy skill should have empty description, got %q", got.Skills[0].Description)
	}
}

func TestRegisterAndListInstances(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.RegisterInstance(ctx, "mcp-aaa"); err != nil {
		t.Fatalf("RegisterInstance: %v", err)
	}
	if err := st.RegisterInstance(ctx, "mcp-bbb"); err != nil {
		t.Fatalf("RegisterInstance: %v", err)
	}

	ids, err := st.ListActiveInstances(ctx)
	if err != nil {
		t.Fatalf("ListActiveInstances: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 active instances, got %d", len(ids))
	}
}

func TestUnregisterInstance(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.RegisterInstance(ctx, "mcp-aaa")
	st.RegisterInstance(ctx, "mcp-bbb")

	if err := st.UnregisterInstance(ctx, "mcp-aaa"); err != nil {
		t.Fatalf("UnregisterInstance: %v", err)
	}

	ids, err := st.ListActiveInstances(ctx)
	if err != nil {
		t.Fatalf("ListActiveInstances: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mcp-bbb" {
		t.Fatalf("expected [mcp-bbb], got %v", ids)
	}
}

func TestCleanupStaleAgents(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// 2つのインスタンスからエージェント登録
	st.UpsertAgent(ctx, domain.Agent{Name: "alive-agent", PaneID: "%1", MCPInstanceID: "mcp-alive"})
	st.UpsertAgent(ctx, domain.Agent{Name: "stale-agent", PaneID: "%2", MCPInstanceID: "mcp-dead"})
	st.UpsertAgent(ctx, domain.Agent{Name: "legacy-agent", PaneID: "%3"}) // MCPInstanceID なし（旧データ）
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "alive-agent", Status: domain.AgentWorkStatusBusy, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(alive-agent): %v", err)
	}
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "stale-agent", Status: domain.AgentWorkStatusIdle, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(stale-agent): %v", err)
	}
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "legacy-agent", Status: domain.AgentWorkStatusIdle, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(legacy-agent): %v", err)
	}

	n, err := st.CleanupStaleAgents(ctx, []string{"mcp-alive"})
	if err != nil {
		t.Fatalf("CleanupStaleAgents: %v", err)
	}
	// Both stale-agent (dead instance) and legacy-agent (NULL instance) are removed.
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}

	agents, _ := st.ListAgents(ctx)
	if len(agents) != 1 {
		t.Fatalf("expected 1 remaining agent, got %d", len(agents))
	}
	if agents[0].Name != "alive-agent" {
		t.Fatalf("expected alive-agent, got %s", agents[0].Name)
	}

	statuses, err := st.ListAgentStatuses(ctx)
	if err != nil {
		t.Fatalf("ListAgentStatuses: %v", err)
	}
	if len(statuses) != 1 || statuses[0].AgentName != "alive-agent" {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestCleanupStaleAgentsNoActiveInstances(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "agent1", PaneID: "%1", MCPInstanceID: "mcp-dead"})
	st.UpsertAgent(ctx, domain.Agent{Name: "legacy", PaneID: "%2"})
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "agent1", Status: domain.AgentWorkStatusBusy, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(agent1): %v", err)
	}
	if err := st.UpsertAgentStatus(ctx, domain.AgentStatus{AgentName: "legacy", Status: domain.AgentWorkStatusIdle, UpdatedAt: "2026-04-02T10:00:00Z"}); err != nil {
		t.Fatalf("UpsertAgentStatus(legacy): %v", err)
	}

	n, err := st.CleanupStaleAgents(ctx, []string{})
	if err != nil {
		t.Fatalf("CleanupStaleAgents: %v", err)
	}
	// Both agents (dead instance + NULL instance) are removed.
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}

	agents, _ := st.ListAgents(ctx)
	if len(agents) != 0 {
		t.Fatalf("expected no remaining agents, got %v", agents)
	}

	statuses, err := st.ListAgentStatuses(ctx)
	if err != nil {
		t.Fatalf("ListAgentStatuses: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("expected no remaining statuses, got %+v", statuses)
	}
}

func TestGetTaskGroup(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.CreateTaskGroup(ctx, domain.TaskGroup{
		ID:        "g-1",
		Label:     "phase3",
		CreatedAt: "2026-04-14T10:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTaskGroup: %v", err)
	}
	group, err := st.GetTaskGroup(ctx, "g-1")
	if err != nil {
		t.Fatalf("GetTaskGroup: %v", err)
	}
	if group.ID != "g-1" || group.Label != "phase3" || group.CreatedAt != "2026-04-14T10:00:00Z" {
		t.Fatalf("unexpected task group: %+v", group)
	}
}

func TestGetTaskGroupNotFound(t *testing.T) {
	st := newTestStore(t)
	_, err := st.GetTaskGroup(context.Background(), "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetTaskGroup error = %v, want ErrNotFound", err)
	}
}

func TestCleanupStaleTasks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a1", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "a2", PaneID: "%2"})

	st.CreateTask(ctx, domain.Task{ID: "t-alive", AgentName: "a1", SenderInstanceID: "mcp-alive", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-stale", AgentName: "a2", SenderInstanceID: "mcp-dead", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-stale-blocked", AgentName: "a2", SenderInstanceID: "mcp-dead", Status: "blocked", SentAt: "2026-01-01T00:30:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-legacy", AgentName: "a1", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-done", AgentName: "a2", SenderInstanceID: "mcp-dead", Status: "completed", SentAt: "2026-01-01T00:00:00Z"})

	n, err := st.CleanupStaleTasks(ctx, []string{"mcp-alive"})
	if err != nil {
		t.Fatalf("CleanupStaleTasks: %v", err)
	}
	// t-stale, t-stale-blocked, and t-legacy are abandoned.
	if n != 3 {
		t.Fatalf("expected 3 abandoned, got %d", n)
	}

	task, _ := st.GetTask(ctx, "t-stale")
	if task.Status != "abandoned" {
		t.Fatalf("expected t-stale to be abandoned, got %s", task.Status)
	}
	blockedTask, _ := st.GetTask(ctx, "t-stale-blocked")
	if blockedTask.Status != "abandoned" {
		t.Fatalf("expected t-stale-blocked to be abandoned, got %s", blockedTask.Status)
	}

	// alive task should remain pending
	aliveTask, _ := st.GetTask(ctx, "t-alive")
	if aliveTask.Status != "pending" {
		t.Fatalf("expected t-alive to remain pending, got %s", aliveTask.Status)
	}

	// legacy task (no sender_instance_id) should now be abandoned
	legacyTask, _ := st.GetTask(ctx, "t-legacy")
	if legacyTask.Status != "abandoned" {
		t.Fatalf("expected t-legacy to be abandoned, got %s", legacyTask.Status)
	}

	// completed task should remain completed
	doneTask, _ := st.GetTask(ctx, "t-done")
	if doneTask.Status != "completed" {
		t.Fatalf("expected t-done to remain completed, got %s", doneTask.Status)
	}
}

func TestCleanupStaleTasksWithoutActiveInstancesAbandonsBlockedTasks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a1", PaneID: "%1"})
	st.CreateTask(ctx, domain.Task{ID: "t-blocked", AgentName: "a1", Status: "blocked", SentAt: "2026-01-01T00:00:00Z"})

	n, err := st.CleanupStaleTasks(ctx, nil)
	if err != nil {
		t.Fatalf("CleanupStaleTasks: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 abandoned, got %d", n)
	}

	task, err := st.GetTask(ctx, "t-blocked")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Status != "abandoned" {
		t.Fatalf("expected t-blocked to be abandoned, got %s", task.Status)
	}
}

func TestAgentMCPInstanceIDRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "x", PaneID: "%1", MCPInstanceID: "mcp-abc123"})
	got, err := st.GetAgent(ctx, "x")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.MCPInstanceID != "mcp-abc123" {
		t.Fatalf("expected mcp_instance_id=mcp-abc123, got %q", got.MCPInstanceID)
	}
}

func TestTaskSenderInstanceIDRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a", PaneID: "%1"})
	st.CreateTask(ctx, domain.Task{ID: "t-1", AgentName: "a", SenderInstanceID: "mcp-xyz789", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	got, err := st.GetTask(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.SenderInstanceID != "mcp-xyz789" {
		t.Fatalf("expected sender_instance_id=mcp-xyz789, got %q", got.SenderInstanceID)
	}
}

func TestDomainAgentFieldCount(t *testing.T) {
	if got := reflect.TypeFor[domain.Agent]().NumField(); got != 6 {
		t.Fatalf("domain.Agent has %d fields (expected 6). Update UpsertAgent/GetAgent/ListAgents scan logic and this constant.", got)
	}
}

func TestDomainTaskFieldCount(t *testing.T) {
	if got := reflect.TypeFor[domain.Task]().NumField(); got != 20 {
		t.Fatalf("domain.Task has %d fields (expected 20). Update GetTask/ListTasks scan logic and this constant.", got)
	}
}

func TestTaskExtendedFieldsRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-extended",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T10:00:00Z",
		ExpiresAt: "2026-04-02T11:00:00Z",
		GroupID:   "g-001",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	progressPct := 60
	progressNote := "tests 4/7 complete"
	if err := st.AcknowledgeTask(ctx, "t-extended", "2026-04-02T10:01:00Z"); err != nil {
		t.Fatalf("AcknowledgeTask: %v", err)
	}
	if err := st.UpdateTaskProgress(ctx, "t-extended", &progressPct, &progressNote, "2026-04-02T10:03:00Z"); err != nil {
		t.Fatalf("UpdateTaskProgress: %v", err)
	}

	got, err := st.GetTask(ctx, "t-extended")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.AcknowledgedAt != "2026-04-02T10:01:00Z" {
		t.Fatalf("AcknowledgedAt = %q", got.AcknowledgedAt)
	}
	if got.ProgressPct == nil || *got.ProgressPct != 60 {
		t.Fatalf("ProgressPct = %v", got.ProgressPct)
	}
	if got.ProgressNote != progressNote {
		t.Fatalf("ProgressNote = %q", got.ProgressNote)
	}
	if got.ProgressUpdatedAt != "2026-04-02T10:03:00Z" {
		t.Fatalf("ProgressUpdatedAt = %q", got.ProgressUpdatedAt)
	}
	if got.ExpiresAt != "2026-04-02T11:00:00Z" {
		t.Fatalf("ExpiresAt = %q", got.ExpiresAt)
	}
	if got.GroupID != "g-001" {
		t.Fatalf("GroupID = %q", got.GroupID)
	}
}

func TestCancelTaskAllowsBlockedStatus(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-blocked",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := st.CancelTask(ctx, "t-blocked", "2026-04-02T10:05:00Z", "no longer needed"); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}

	got, err := st.GetTask(ctx, "t-blocked")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != domain.TaskStatusCancelled {
		t.Fatalf("Status = %q", got.Status)
	}
	if got.CancelledAt != "2026-04-02T10:05:00Z" || got.CompletedAt != "2026-04-02T10:05:00Z" {
		t.Fatalf("timestamps = cancelled:%q completed:%q", got.CancelledAt, got.CompletedAt)
	}
	if got.CancelReason != "no longer needed" {
		t.Fatalf("CancelReason = %q", got.CancelReason)
	}
}

func TestCancelTaskRejectsCompletedAndFailedStatuses(t *testing.T) {
	tests := []struct {
		name   string
		taskID string
		status domain.TaskStatus
	}{
		{name: "completed", taskID: "t-completed", status: domain.TaskStatusCompleted},
		{name: "failed", taskID: "t-failed", status: domain.TaskStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore(t)
			ctx := context.Background()

			if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
				t.Fatalf("UpsertAgent: %v", err)
			}
			if err := st.CreateTask(ctx, domain.Task{
				ID:        tt.taskID,
				AgentName: "worker",
				Status:    tt.status,
				SentAt:    "2026-04-02T10:00:00Z",
			}); err != nil {
				t.Fatalf("CreateTask: %v", err)
			}

			err := st.CancelTask(ctx, tt.taskID, "2026-04-02T10:05:00Z", "should fail")
			if err == nil || !strings.Contains(err.Error(), string(tt.status)) {
				t.Fatalf("CancelTask error = %v, want status %q", err, tt.status)
			}

			got, getErr := st.GetTask(ctx, tt.taskID)
			if getErr != nil {
				t.Fatalf("GetTask: %v", getErr)
			}
			if got.Status != tt.status {
				t.Fatalf("Status = %q, want %q", got.Status, tt.status)
			}
		})
	}
}

func TestCreateTaskWithDependenciesAndActivation(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-dep-done",
		AgentName: "worker",
		Status:    domain.TaskStatusCompleted,
		SentAt:    "2026-04-02T09:50:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(dep-done): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-dep-open",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T09:55:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(dep-open): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-ready",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}, []string{"t-dep-done"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(ready): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-blocked",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:01:00Z",
	}, []string{"t-dep-open"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(blocked): %v", err)
	}

	dependencyTaskIDs, err := st.GetTaskDependencies(ctx, "t-ready")
	if err != nil {
		t.Fatalf("GetTaskDependencies: %v", err)
	}
	if !reflect.DeepEqual(dependencyTaskIDs, []string{"t-dep-done"}) {
		t.Fatalf("dependencyTaskIDs = %v", dependencyTaskIDs)
	}

	activated, stillBlocked, err := st.ActivateReadyTasks(ctx, "2026-04-02T10:05:00Z", "worker")
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(activated) != 1 || activated[0].ID != "t-ready" {
		t.Fatalf("activated = %+v", activated)
	}
	if stillBlocked != 1 {
		t.Fatalf("stillBlocked = %d, want 1", stillBlocked)
	}
	readyTask, err := st.GetTask(ctx, "t-ready")
	if err != nil {
		t.Fatalf("GetTask(t-ready): %v", err)
	}
	if readyTask.Status != domain.TaskStatusPending {
		t.Fatalf("ready task status = %q", readyTask.Status)
	}
	blockedTask, err := st.GetTask(ctx, "t-blocked")
	if err != nil {
		t.Fatalf("GetTask(t-blocked): %v", err)
	}
	if blockedTask.Status != domain.TaskStatusBlocked {
		t.Fatalf("blocked task status = %q", blockedTask.Status)
	}
}

func TestCreateTaskWithDependenciesRejectsMissingDependency(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}

	err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-invalid",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}, []string{"t-missing"})
	if err == nil || !strings.Contains(err.Error(), "dependency task \"t-missing\" not found") {
		t.Fatalf("CreateTaskWithDependencies error = %v", err)
	}
}

func TestCreateTaskWithDependenciesRejectsCycle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-root",
		AgentName: "worker",
		Status:    domain.TaskStatusCompleted,
		SentAt:    "2026-04-02T09:50:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(t-root): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-child",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}, []string{"t-root"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(t-child): %v", err)
	}

	err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-root",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:10:00Z",
	}, []string{"t-child"})
	if err == nil || !strings.Contains(err.Error(), "would create a cycle") {
		t.Fatalf("CreateTaskWithDependencies cycle error = %v", err)
	}
}

func TestTaskDependenciesCascadeWhenForeignKeysEnabled(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-dependency",
		AgentName: "worker",
		Status:    domain.TaskStatusCompleted,
		SentAt:    "2026-04-02T09:50:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(t-dependency): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-blocked",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}, []string{"t-dependency"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `DELETE FROM tasks WHERE task_id = ?`, "t-dependency"); err != nil {
		t.Fatalf("delete dependency task: %v", err)
	}

	var depCount int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_dependencies WHERE task_id = ?`, "t-blocked").Scan(&depCount); err != nil {
		t.Fatalf("count task dependencies: %v", err)
	}
	if depCount != 0 {
		t.Fatalf("dependency rows = %d, want 0 after ON DELETE CASCADE", depCount)
	}
}

func TestCreateTaskStoresNullAssigneePaneIDWhenEmpty(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-empty-pane",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T10:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var assignee sql.NullString
	if err := st.db.QueryRowContext(ctx, `SELECT assignee_pane_id FROM tasks WHERE task_id = ?`, "t-empty-pane").Scan(&assignee); err != nil {
		t.Fatalf("query assignee_pane_id: %v", err)
	}
	if assignee.Valid {
		t.Fatalf("assignee_pane_id = %q, want NULL", assignee.String)
	}
}

func TestTaskSelectColumnsMatchesTaskFieldCount(t *testing.T) {
	got := strings.Count(taskSelectColumns, ",") + 1
	want := reflect.TypeFor[domain.Task]().NumField()
	if want != 20 {
		t.Fatalf("domain.Task field count = %d, want 20", want)
	}
	if got != want {
		t.Fatalf("taskSelectColumns count = %d, want %d", got, want)
	}
}

func TestActivateReadyTasksRestoresNowSessionForReactivatedTask(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:               "t-dep-done",
		AgentName:        "worker",
		Status:           domain.TaskStatusCompleted,
		SentAt:           "2026-04-02T09:50:00Z",
		SenderInstanceID: "mcp-old",
	}); err != nil {
		t.Fatalf("CreateTask(dep-done): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:               "t-reactivate",
		AgentName:        "worker",
		Status:           domain.TaskStatusBlocked,
		SentAt:           "2026-04-02T10:00:00Z",
		SenderInstanceID: "mcp-old",
	}, []string{"t-dep-done"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(t-reactivate): %v", err)
	}
	if err := st.EndSessionByInstanceID(ctx, "mcp-old"); err != nil {
		t.Fatalf("EndSessionByInstanceID: %v", err)
	}

	before, err := st.GetTask(ctx, "t-reactivate")
	if err != nil {
		t.Fatalf("GetTask(before): %v", err)
	}
	if before.IsNowSession {
		t.Fatal("blocked task should be marked old-session after EndSessionByInstanceID")
	}

	activated, stillBlocked, err := st.ActivateReadyTasks(ctx, "2026-04-02T10:05:00Z", "worker")
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if stillBlocked != 0 {
		t.Fatalf("stillBlocked = %d, want 0", stillBlocked)
	}
	if len(activated) != 1 || activated[0].ID != "t-reactivate" {
		t.Fatalf("activated = %+v", activated)
	}
	if !activated[0].IsNowSession {
		t.Fatal("activated task should be returned as current-session")
	}

	after, err := st.GetTask(ctx, "t-reactivate")
	if err != nil {
		t.Fatalf("GetTask(after): %v", err)
	}
	if after.Status != domain.TaskStatusPending {
		t.Fatalf("status = %q, want %q", after.Status, domain.TaskStatusPending)
	}
	if !after.IsNowSession {
		t.Fatal("reactivated task should be marked current-session")
	}

	nowSession := true
	filtered, err := st.ListTasks(ctx, domain.TaskFilter{
		Status:       domain.TaskStatusFilterAll,
		IsNowSession: &nowSession,
	})
	if err != nil {
		t.Fatalf("ListTasks(now session): %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "t-reactivate" {
		t.Fatalf("now-session tasks = %+v, want only t-reactivate", filtered)
	}
}

func TestActivateReadyTasksCancelsBrokenDependencies(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-dep-cancelled",
		AgentName: "worker",
		Status:    domain.TaskStatusCancelled,
		SentAt:    "2026-04-02T09:50:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(dep-cancelled): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-broken",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
	}, []string{"t-dep-cancelled"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(broken): %v", err)
	}

	activated, stillBlocked, err := st.ActivateReadyTasks(ctx, "2026-04-02T10:05:00Z", "worker")
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(activated) != 0 {
		t.Fatalf("activated = %+v", activated)
	}
	if stillBlocked != 0 {
		t.Fatalf("stillBlocked = %d, want 0", stillBlocked)
	}

	brokenTask, err := st.GetTask(ctx, "t-broken")
	if err != nil {
		t.Fatalf("GetTask(t-broken): %v", err)
	}
	if brokenTask.Status != domain.TaskStatusCancelled {
		t.Fatalf("broken task status = %q", brokenTask.Status)
	}
	if brokenTask.CancelReason == "" {
		t.Fatal("CancelReason should be populated")
	}
}

func TestActivateReadyTasksExpiresBlockedTaskBeforeDependencyWait(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent(worker): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-dep-open",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T09:50:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(dep-open): %v", err)
	}
	if err := st.CreateTaskWithDependencies(ctx, domain.Task{
		ID:        "t-expired",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
		ExpiresAt: "2026-04-02T10:04:00Z",
	}, []string{"t-dep-open"}); err != nil {
		t.Fatalf("CreateTaskWithDependencies(expired): %v", err)
	}

	activated, stillBlocked, err := st.ActivateReadyTasks(ctx, "2026-04-02T10:05:00Z", "worker")
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(activated) != 0 {
		t.Fatalf("activated = %+v", activated)
	}
	if stillBlocked != 0 {
		t.Fatalf("stillBlocked = %d, want 0", stillBlocked)
	}

	expiredTask, err := st.GetTask(ctx, "t-expired")
	if err != nil {
		t.Fatalf("GetTask(t-expired): %v", err)
	}
	if expiredTask.Status != domain.TaskStatusExpired {
		t.Fatalf("expired task status = %q", expiredTask.Status)
	}
}

func TestActivateReadyTasksAgentFilterBoundaries(t *testing.T) {
	tests := []struct {
		name              string
		agentFilter       string
		wantActivatedIDs  []string
		wantStillBlocked  int
		wantPendingByTask map[string]bool
	}{
		{
			name:             "empty agent filter activates all ready blocked tasks",
			agentFilter:      "",
			wantActivatedIDs: []string{"t-ready-a", "t-ready-b"},
			wantStillBlocked: 2,
			wantPendingByTask: map[string]bool{
				"t-ready-a": true,
				"t-ready-b": true,
				"t-wait-a":  false,
				"t-wait-b":  false,
			},
		},
		{
			name:             "named agent filter scopes activation",
			agentFilter:      "worker-a",
			wantActivatedIDs: []string{"t-ready-a"},
			wantStillBlocked: 1,
			wantPendingByTask: map[string]bool{
				"t-ready-a": true,
				"t-ready-b": false,
				"t-wait-a":  false,
				"t-wait-b":  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore(t)
			ctx := context.Background()

			for _, agent := range []domain.Agent{
				{Name: "worker-a", PaneID: "%1"},
				{Name: "worker-b", PaneID: "%2"},
			} {
				if err := st.UpsertAgent(ctx, agent); err != nil {
					t.Fatalf("UpsertAgent(%s): %v", agent.Name, err)
				}
			}

			for _, task := range []domain.Task{
				{ID: "t-dep-done-a", AgentName: "worker-a", Status: domain.TaskStatusCompleted, SentAt: "2026-04-02T09:50:00Z"},
				{ID: "t-dep-open-a", AgentName: "worker-a", Status: domain.TaskStatusPending, SentAt: "2026-04-02T09:51:00Z"},
				{ID: "t-dep-done-b", AgentName: "worker-b", Status: domain.TaskStatusCompleted, SentAt: "2026-04-02T09:52:00Z"},
				{ID: "t-dep-open-b", AgentName: "worker-b", Status: domain.TaskStatusPending, SentAt: "2026-04-02T09:53:00Z"},
			} {
				if err := st.CreateTask(ctx, task); err != nil {
					t.Fatalf("CreateTask(%s): %v", task.ID, err)
				}
			}

			for _, blocked := range []struct {
				taskID       string
				agentName    string
				dependencies []string
			}{
				{taskID: "t-ready-a", agentName: "worker-a", dependencies: []string{"t-dep-done-a"}},
				{taskID: "t-wait-a", agentName: "worker-a", dependencies: []string{"t-dep-open-a"}},
				{taskID: "t-ready-b", agentName: "worker-b", dependencies: []string{"t-dep-done-b"}},
				{taskID: "t-wait-b", agentName: "worker-b", dependencies: []string{"t-dep-open-b"}},
			} {
				if err := st.CreateTaskWithDependencies(ctx, domain.Task{
					ID:        blocked.taskID,
					AgentName: blocked.agentName,
					Status:    domain.TaskStatusBlocked,
					SentAt:    "2026-04-02T10:00:00Z",
				}, blocked.dependencies); err != nil {
					t.Fatalf("CreateTaskWithDependencies(%s): %v", blocked.taskID, err)
				}
			}

			activated, stillBlocked, err := st.ActivateReadyTasks(ctx, "2026-04-02T10:05:00Z", tt.agentFilter)
			if err != nil {
				t.Fatalf("ActivateReadyTasks(%q): %v", tt.agentFilter, err)
			}
			if stillBlocked != tt.wantStillBlocked {
				t.Fatalf("stillBlocked = %d, want %d", stillBlocked, tt.wantStillBlocked)
			}

			gotActivatedIDs := make([]string, 0, len(activated))
			for _, task := range activated {
				gotActivatedIDs = append(gotActivatedIDs, task.ID)
			}
			sort.Strings(gotActivatedIDs)
			if !reflect.DeepEqual(gotActivatedIDs, tt.wantActivatedIDs) {
				t.Fatalf("activated IDs = %v, want %v", gotActivatedIDs, tt.wantActivatedIDs)
			}

			for taskID, wantPending := range tt.wantPendingByTask {
				task, err := st.GetTask(ctx, taskID)
				if err != nil {
					t.Fatalf("GetTask(%s): %v", taskID, err)
				}
				gotPending := task.Status == domain.TaskStatusPending
				if gotPending != wantPending {
					t.Errorf("%s pending = %v, want %v (status=%q)", taskID, gotPending, wantPending, task.Status)
				}
			}
		})
	}
}

func TestExpirePendingTasks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.UpsertAgent(ctx, domain.Agent{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-expired",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T10:00:00Z",
		ExpiresAt: "2026-04-02T10:05:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(expired): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-active",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T10:00:00Z",
		ExpiresAt: "2026-04-02T10:20:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(active): %v", err)
	}
	if err := st.CreateTask(ctx, domain.Task{
		ID:        "t-blocked-expired",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		SentAt:    "2026-04-02T10:00:00Z",
		ExpiresAt: "2026-04-02T10:03:00Z",
	}); err != nil {
		t.Fatalf("CreateTask(blocked-expired): %v", err)
	}

	expired, err := st.ExpirePendingTasks(ctx, "2026-04-02T10:10:00Z")
	if err != nil {
		t.Fatalf("ExpirePendingTasks: %v", err)
	}
	if expired != 2 {
		t.Fatalf("expired = %d, want 2", expired)
	}

	expiredTask, err := st.GetTask(ctx, "t-expired")
	if err != nil {
		t.Fatalf("GetTask(t-expired): %v", err)
	}
	if expiredTask.Status != domain.TaskStatusExpired {
		t.Fatalf("expired task status = %q", expiredTask.Status)
	}

	blockedExpiredTask, err := st.GetTask(ctx, "t-blocked-expired")
	if err != nil {
		t.Fatalf("GetTask(t-blocked-expired): %v", err)
	}
	if blockedExpiredTask.Status != domain.TaskStatusExpired {
		t.Fatalf("blocked expired task status = %q", blockedExpiredTask.Status)
	}

	activeTask, err := st.GetTask(ctx, "t-active")
	if err != nil {
		t.Fatalf("GetTask(t-active): %v", err)
	}
	if activeTask.Status != domain.TaskStatusPending {
		t.Fatalf("active task status = %q", activeTask.Status)
	}
}

func TestEndSessionByInstanceID(t *testing.T) {
	tests := []struct {
		name           string
		instanceID     string
		setupTasks     []domain.Task
		setupAgents    []domain.Agent
		wantNowSession map[string]bool // taskID -> expected is_now_session
	}{
		{
			name:       "matching sender_instance_id tasks updated",
			instanceID: "mcp-end",
			setupAgents: []domain.Agent{
				{Name: "a1", PaneID: "%1"},
			},
			setupTasks: []domain.Task{
				{ID: "t-match", AgentName: "a1", SenderInstanceID: "mcp-end", Status: "pending", SentAt: "2026-01-01T00:00:00Z"},
				{ID: "t-other", AgentName: "a1", SenderInstanceID: "mcp-other", Status: "pending", SentAt: "2026-01-01T00:00:00Z"},
			},
			wantNowSession: map[string]bool{
				"t-match": false,
				"t-other": true,
			},
		},
		{
			name:       "zero matching tasks causes no error",
			instanceID: "mcp-nonexistent",
			setupAgents: []domain.Agent{
				{Name: "a1", PaneID: "%1"},
			},
			setupTasks: []domain.Task{
				{ID: "t-1", AgentName: "a1", SenderInstanceID: "mcp-alive", Status: "pending", SentAt: "2026-01-01T00:00:00Z"},
			},
			wantNowSession: map[string]bool{
				"t-1": true,
			},
		},
		{
			name:       "other tasks not affected",
			instanceID: "mcp-end",
			setupAgents: []domain.Agent{
				{Name: "a1", PaneID: "%1"},
				{Name: "a2", PaneID: "%2"},
			},
			setupTasks: []domain.Task{
				{ID: "t-end1", AgentName: "a1", SenderInstanceID: "mcp-end", Status: "pending", SentAt: "2026-01-01T00:00:00Z"},
				{ID: "t-end2", AgentName: "a2", SenderInstanceID: "mcp-end", Status: "completed", SentAt: "2026-01-01T00:00:00Z"},
				{ID: "t-keep", AgentName: "a1", SenderInstanceID: "mcp-alive", Status: "pending", SentAt: "2026-01-01T00:00:00Z"},
			},
			wantNowSession: map[string]bool{
				"t-end1": false,
				"t-end2": false,
				"t-keep": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestStore(t)
			ctx := context.Background()

			for _, agent := range tt.setupAgents {
				if err := st.UpsertAgent(ctx, agent); err != nil {
					t.Fatalf("UpsertAgent(%s): %v", agent.Name, err)
				}
			}
			for _, task := range tt.setupTasks {
				if err := st.CreateTask(ctx, task); err != nil {
					t.Fatalf("CreateTask(%s): %v", task.ID, err)
				}
			}

			if err := st.EndSessionByInstanceID(ctx, tt.instanceID); err != nil {
				t.Fatalf("EndSessionByInstanceID(%s): %v", tt.instanceID, err)
			}

			for taskID, wantNow := range tt.wantNowSession {
				got, err := st.GetTask(ctx, taskID)
				if err != nil {
					t.Fatalf("GetTask(%s): %v", taskID, err)
				}
				if got.IsNowSession != wantNow {
					t.Errorf("task %s: IsNowSession = %v, want %v", taskID, got.IsNowSession, wantNow)
				}
			}
		})
	}
}

func TestListTasksIsNowSessionFilter(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a1", PaneID: "%1"})

	// Create tasks: t-now is is_now_session=1 (default from CreateTask), t-old needs is_now_session=0
	st.CreateTask(ctx, domain.Task{ID: "t-now", AgentName: "a1", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-old", AgentName: "a1", SenderInstanceID: "mcp-end", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	// End session to mark t-old as is_now_session=0
	st.EndSessionByInstanceID(ctx, "mcp-end")

	boolTrue := true
	boolFalse := false

	tests := []struct {
		name        string
		filter      domain.TaskFilter
		wantCount   int
		wantTaskIDs []string
	}{
		{
			name:      "nil IsNowSession returns all",
			filter:    domain.TaskFilter{Status: domain.TaskStatusFilterAll, IsNowSession: nil},
			wantCount: 2,
		},
		{
			name:        "true returns is_now_session=1 only",
			filter:      domain.TaskFilter{Status: domain.TaskStatusFilterAll, IsNowSession: &boolTrue},
			wantCount:   1,
			wantTaskIDs: []string{"t-now"},
		},
		{
			name:        "false returns is_now_session=0 only",
			filter:      domain.TaskFilter{Status: domain.TaskStatusFilterAll, IsNowSession: &boolFalse},
			wantCount:   1,
			wantTaskIDs: []string{"t-old"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := st.ListTasks(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(tasks) != tt.wantCount {
				t.Fatalf("got %d tasks, want %d", len(tasks), tt.wantCount)
			}
			if tt.wantTaskIDs != nil {
				for i, wantID := range tt.wantTaskIDs {
					if tasks[i].ID != wantID {
						t.Errorf("tasks[%d].ID = %q, want %q", i, tasks[i].ID, wantID)
					}
				}
			}
		})
	}
}

func TestGetMessage(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// 事前にメッセージを保存
	if err := st.SaveMessage(ctx, domain.TaskMessage{ID: "m-001", Content: "hello world", CreatedAt: "2026-03-07T10:00:00Z"}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}

	tests := []struct {
		name    string
		id      string
		want    domain.TaskMessage
		wantErr error
	}{
		{
			name: "found",
			id:   "m-001",
			want: domain.TaskMessage{ID: "m-001", Content: "hello world", CreatedAt: "2026-03-07T10:00:00Z"},
		},
		{
			name:    "not found",
			id:      "m-nonexistent",
			wantErr: domain.ErrNotFound,
		},
		{
			name:    "empty id",
			id:      "",
			wantErr: domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := st.GetMessage(ctx, tt.id)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GetMessage(%q) error = %v, want %v", tt.id, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetMessage(%q): %v", tt.id, err)
			}
			want := normalizeTaskMessage(tt.want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("GetMessage(%q) = %+v, want %+v", tt.id, got, want)
			}
		})
	}
}

func TestDeleteMessage(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.SaveMessage(ctx, domain.TaskMessage{ID: "m-001", Content: "hello world", CreatedAt: "2026-03-07T10:00:00Z"}); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if err := st.DeleteMessage(ctx, "m-001"); err != nil {
		t.Fatalf("DeleteMessage(existing): %v", err)
	}
	if _, err := st.GetMessage(ctx, "m-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetMessage(after delete) error = %v, want ErrNotFound", err)
	}
	if err := st.DeleteMessage(ctx, "m-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteMessage(missing) error = %v, want ErrNotFound", err)
	}
}

func TestDeleteResponse(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.SaveResponse(ctx, domain.TaskMessage{ID: "r-001", Content: "done", CreatedAt: "2026-03-07T10:00:00Z"}); err != nil {
		t.Fatalf("SaveResponse: %v", err)
	}
	if err := st.DeleteResponse(ctx, "r-001"); err != nil {
		t.Fatalf("DeleteResponse(existing): %v", err)
	}
	if _, err := st.GetResponse(ctx, "r-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetResponse(after delete) error = %v, want ErrNotFound", err)
	}
	if err := st.DeleteResponse(ctx, "r-001"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("DeleteResponse(missing) error = %v, want ErrNotFound", err)
	}
}

func TestGetTaskBySendMessageID(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	st.UpsertAgent(ctx, domain.Agent{Name: "codex", PaneID: "%1"})
	st.SaveMessage(ctx, domain.TaskMessage{ID: "m-001", Content: "hello", CreatedAt: "2026-03-07T10:00:00Z"})
	st.CreateTask(ctx, domain.Task{
		ID:            "t-001",
		AgentName:     "codex",
		SendMessageID: "m-001",
		Status:        "pending",
		SentAt:        "2026-03-07T10:00:00Z",
	})

	tests := []struct {
		name          string
		sendMessageID string
		wantTaskID    string
		wantErr       error
	}{
		{
			name:          "found",
			sendMessageID: "m-001",
			wantTaskID:    "t-001",
		},
		{
			name:          "not found",
			sendMessageID: "m-nonexistent",
			wantErr:       domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := st.GetTaskBySendMessageID(ctx, tt.sendMessageID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GetTaskBySendMessageID(%q) error = %v, want %v", tt.sendMessageID, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetTaskBySendMessageID(%q): %v", tt.sendMessageID, err)
			}
			if got.ID != tt.wantTaskID {
				t.Errorf("got task_id = %q, want %q", got.ID, tt.wantTaskID)
			}
			if got.SendMessageID != tt.sendMessageID {
				t.Errorf("got send_message_id = %q, want %q", got.SendMessageID, tt.sendMessageID)
			}
		})
	}
}

func TestNormalizeDBPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "clean relative path", input: filepath.Join("sub", "..", "test.db"), want: filepath.Clean(filepath.Join("sub", "..", "test.db"))},
		{name: "empty path", input: "", wantErr: true},
		{name: "whitespace path", input: "   ", wantErr: true},
		{name: "query delimiter", input: "test?.db", wantErr: true},
		{name: "fragment delimiter", input: "test#.db", wantErr: true},
		{name: "nul byte", input: "test\x00.db", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDBPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeDBPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("normalizeDBPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
