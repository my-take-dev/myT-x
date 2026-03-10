package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
		Skills: []string{"Go", "API設計"},
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
	if len(got.Skills) != 2 || got.Skills[0] != "Go" || got.Skills[1] != "API設計" {
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
		Label:          "テスト",
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
	completedAt := "2026-03-08T12:34:56Z"

	if err := st.CompleteTask(ctx, "t-001", "done", completedAt); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	got, _ := st.GetTask(ctx, "t-001")
	if got.Status != "completed" || got.Notes != "done" || got.CompletedAt != completedAt {
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

	err := st.CompleteTask(ctx, "t-001", "done again", "2026-03-08T12:34:56Z")
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

	if err := st.MarkTaskFailed(ctx, "t-001", "delivery failed"); err != nil {
		t.Fatalf("MarkTaskFailed: %v", err)
	}

	got, err := st.GetTask(ctx, "t-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "failed" || got.Notes != "delivery failed" || got.CompletedAt == "" {
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
