package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

	if err := st.DeleteAgentsByPaneID(ctx, "%1"); err != nil {
		t.Fatalf("DeleteAgentsByPaneID: %v", err)
	}

	agents, _ := st.ListAgents(ctx)
	if len(agents) != 1 || agents[0].Name != "c" {
		t.Errorf("got %+v", agents)
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
		{"all", domain.TaskFilter{Status: "all"}, 5},
		{"pending only", domain.TaskFilter{Status: "pending"}, 2},
		{"completed only", domain.TaskFilter{Status: "completed"}, 1},
		{"failed only", domain.TaskFilter{Status: "failed"}, 1},
		{"abandoned only", domain.TaskFilter{Status: "abandoned"}, 1},
		{"agent a pending", domain.TaskFilter{Status: "pending", AgentName: "a"}, 1},
		{"agent b all", domain.TaskFilter{Status: "all", AgentName: "b"}, 2},
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
}

func TestCleanupStaleAgentsNoActiveInstances(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "agent1", PaneID: "%1", MCPInstanceID: "mcp-dead"})
	st.UpsertAgent(ctx, domain.Agent{Name: "legacy", PaneID: "%2"})

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
}

func TestCleanupStaleTasks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	st.UpsertAgent(ctx, domain.Agent{Name: "a1", PaneID: "%1"})
	st.UpsertAgent(ctx, domain.Agent{Name: "a2", PaneID: "%2"})

	st.CreateTask(ctx, domain.Task{ID: "t-alive", AgentName: "a1", SenderInstanceID: "mcp-alive", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-stale", AgentName: "a2", SenderInstanceID: "mcp-dead", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-legacy", AgentName: "a1", Status: "pending", SentAt: "2026-01-01T00:00:00Z"})
	st.CreateTask(ctx, domain.Task{ID: "t-done", AgentName: "a2", SenderInstanceID: "mcp-dead", Status: "completed", SentAt: "2026-01-01T00:00:00Z"})

	n, err := st.CleanupStaleTasks(ctx, []string{"mcp-alive"})
	if err != nil {
		t.Fatalf("CleanupStaleTasks: %v", err)
	}
	// Both t-stale (dead instance) and t-legacy (NULL instance) are abandoned.
	if n != 2 {
		t.Fatalf("expected 2 abandoned, got %d", n)
	}

	task, _ := st.GetTask(ctx, "t-stale")
	if task.Status != "abandoned" {
		t.Fatalf("expected t-stale to be abandoned, got %s", task.Status)
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
	if got := reflect.TypeOf(domain.Agent{}).NumField(); got != 6 {
		t.Fatalf("domain.Agent has %d fields (expected 6). Update UpsertAgent/GetAgent/ListAgents scan logic and this constant.", got)
	}
}

func TestDomainTaskFieldCount(t *testing.T) {
	if got := reflect.TypeOf(domain.Task{}).NumField(); got != 12 {
		t.Fatalf("domain.Task has %d fields (expected 12). Update GetTask/ListTasks scan logic and this constant.", got)
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
			filter:    domain.TaskFilter{Status: "all", IsNowSession: nil},
			wantCount: 2,
		},
		{
			name:        "true returns is_now_session=1 only",
			filter:      domain.TaskFilter{Status: "all", IsNowSession: &boolTrue},
			wantCount:   1,
			wantTaskIDs: []string{"t-now"},
		},
		{
			name:        "false returns is_now_session=0 only",
			filter:      domain.TaskFilter{Status: "all", IsNowSession: &boolFalse},
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
