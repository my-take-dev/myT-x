package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

type testResolver struct {
	paneID string
	err    error
}

func (r testResolver) GetPaneID(context.Context) (string, error) {
	return r.paneID, r.err
}

type testAgentRepo struct {
	deleteCalls []string
}

func (r *testAgentRepo) UpsertAgent(context.Context, domain.Agent) error { return nil }
func (r *testAgentRepo) GetAgent(context.Context, string) (domain.Agent, error) {
	return domain.Agent{}, domain.ErrNotFound
}
func (r *testAgentRepo) GetAgentByPaneID(context.Context, string) (domain.Agent, error) {
	return domain.Agent{}, domain.ErrNotFound
}
func (r *testAgentRepo) ListAgents(context.Context) ([]domain.Agent, error) { return nil, nil }
func (r *testAgentRepo) DeleteAgentsByPaneID(_ context.Context, paneID string) error {
	r.deleteCalls = append(r.deleteCalls, paneID)
	return nil
}

type testTaskRepo struct {
	abandonCalls    []string
	endSessionCalls []string
}

func (r *testTaskRepo) CreateTask(context.Context, domain.Task) error           { return nil }
func (r *testTaskRepo) CreateTaskGroup(context.Context, domain.TaskGroup) error { return nil }
func (r *testTaskRepo) DeleteTaskGroup(context.Context, string) error           { return nil }
func (r *testTaskRepo) CreateTaskWithDependencies(context.Context, domain.Task, []string) error {
	return nil
}
func (r *testTaskRepo) GetTask(context.Context, string) (domain.Task, error) {
	return domain.Task{}, domain.ErrNotFound
}
func (r *testTaskRepo) GetTaskDependencies(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r *testTaskRepo) ListTasks(context.Context, domain.TaskFilter) ([]domain.Task, error) {
	return nil, nil
}
func (r *testTaskRepo) ActivateReadyTasks(context.Context, string, string) ([]domain.Task, int, error) {
	return nil, 0, nil
}
func (r *testTaskRepo) CompleteTask(context.Context, string, string, string) error { return nil }
func (r *testTaskRepo) MarkTaskFailed(context.Context, string) error               { return nil }
func (r *testTaskRepo) AcknowledgeTask(context.Context, string, string) error      { return nil }
func (r *testTaskRepo) CancelTask(context.Context, string, string, string) error   { return nil }
func (r *testTaskRepo) UpdateTaskProgress(context.Context, string, *int, *string, string) error {
	return nil
}
func (r *testTaskRepo) ExpirePendingTasks(context.Context, string) (int64, error) { return 0, nil }
func (r *testTaskRepo) AbandonTasksByPaneID(_ context.Context, paneID string) error {
	r.abandonCalls = append(r.abandonCalls, paneID)
	return nil
}
func (r *testTaskRepo) EndSessionByInstanceID(_ context.Context, instanceID string) error {
	r.endSessionCalls = append(r.endSessionCalls, instanceID)
	return nil
}
func (r *testTaskRepo) GetTaskBySendMessageID(context.Context, string) (domain.Task, error) {
	return domain.Task{}, domain.ErrNotFound
}

func TestRuntimeStartWarnsWhenPaneIDIsEmpty(t *testing.T) {
	var logs bytes.Buffer
	rt := &Runtime{
		cfg: Config{
			Logger: log.New(&logs, "", 0),
		},
		resolver: testResolver{paneID: ""},
	}

	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.Contains(logs.String(), "自ペインID取得結果が空です") {
		t.Fatalf("expected empty pane warning, got %q", logs.String())
	}
	if rt.selfPane != "" {
		t.Fatalf("selfPane = %q, want empty", rt.selfPane)
	}
}

func TestRuntimeCloseWarnsWhenSelfPaneIsUnknown(t *testing.T) {
	var logs bytes.Buffer
	agentRepo := &testAgentRepo{}
	taskRepo := &testTaskRepo{}
	rt := &Runtime{
		cfg: Config{
			Logger: log.New(&logs, "", 0),
		},
		agentRepo: agentRepo,
		taskRepo:  taskRepo,
		started:   true,
	}

	if err := rt.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !strings.Contains(logs.String(), "自ペインID不明のため終了時クリーンアップをスキップします") {
		t.Fatalf("expected cleanup warning, got %q", logs.String())
	}
	if len(agentRepo.deleteCalls) != 0 || len(taskRepo.abandonCalls) != 0 {
		t.Fatalf("cleanup should be skipped: deleteCalls=%v abandonCalls=%v", agentRepo.deleteCalls, taskRepo.abandonCalls)
	}
}

func TestRuntimeStartLogsResolverError(t *testing.T) {
	var logs bytes.Buffer
	rt := &Runtime{
		cfg: Config{
			Logger: log.New(&logs, "", 0),
		},
		resolver: testResolver{err: errors.New("tmux missing")},
	}

	if err := rt.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.Contains(logs.String(), "自ペインID取得に失敗") {
		t.Fatalf("expected resolver failure log, got %q", logs.String())
	}
}

type noopPaneOps struct{}

func (noopPaneOps) SendKeys(context.Context, string, string) error                 { return nil }
func (noopPaneOps) ListPanes(context.Context) ([]domain.PaneInfo, error)           { return nil, nil }
func (noopPaneOps) CapturePaneOutput(context.Context, string, int) (string, error) { return "", nil }
func (noopPaneOps) SetPaneTitle(context.Context, string, string) error             { return nil }
func (noopPaneOps) SplitPane(context.Context, string, bool) (string, error)        { return "%9", nil }
func (noopPaneOps) SendKeysPaste(context.Context, string, string) error            { return nil }

type testStatusRepo struct{}

func (testStatusRepo) UpsertAgentStatus(context.Context, domain.AgentStatus) error { return nil }
func (testStatusRepo) GetAgentStatus(context.Context, string) (domain.AgentStatus, error) {
	return domain.AgentStatus{}, domain.ErrNotFound
}
func (testStatusRepo) ListAgentStatuses(context.Context) ([]domain.AgentStatus, error) {
	return nil, nil
}

type testMessageRepo struct{}

func (testMessageRepo) SaveMessage(context.Context, domain.TaskMessage) error { return nil }
func (testMessageRepo) SaveResponse(context.Context, domain.TaskMessage) error {
	return nil
}
func (testMessageRepo) GetMessage(context.Context, string) (domain.TaskMessage, error) {
	return domain.TaskMessage{}, domain.ErrNotFound
}
func (testMessageRepo) GetResponse(context.Context, string) (domain.TaskMessage, error) {
	return domain.TaskMessage{}, domain.ErrNotFound
}

func TestNewRuntimeCreatesFallbackStatusRepoWhenNotInjected(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), ".myT-x", "orchestrator.db")
	paneOps := noopPaneOps{}
	rt, err := NewRuntime(Config{
		DBPath:        dbPath,
		Logger:        log.New(io.Discard, "", 0),
		AgentRepo:     &testAgentRepo{},
		TaskRepo:      &testTaskRepo{},
		MessageRepo:   testMessageRepo{},
		Sender:        paneOps,
		Lister:        paneOps,
		Capturer:      paneOps,
		SelfResolver:  testResolver{paneID: "%1"},
		TitleSetter:   paneOps,
		Splitter:      paneOps,
		PasteSender:   paneOps,
		ServerName:    "test-server",
		ServerVersion: "test-version",
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	defer func() {
		_ = rt.closeResources(context.Background())
	}()
	if rt.store == nil {
		t.Fatal("expected fallback store to be created for missing status repository")
	}
}

func TestNewRuntimeUsesInjectedStatusRepoWithoutFallbackStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), ".myT-x", "orchestrator.db")
	paneOps := noopPaneOps{}
	rt, err := NewRuntime(Config{
		DBPath:          dbPath,
		Logger:          log.New(io.Discard, "", 0),
		AgentRepo:       &testAgentRepo{},
		AgentStatusRepo: testStatusRepo{},
		TaskRepo:        &testTaskRepo{},
		MessageRepo:     testMessageRepo{},
		Sender:          paneOps,
		Lister:          paneOps,
		Capturer:        paneOps,
		SelfResolver:    testResolver{paneID: "%1"},
		TitleSetter:     paneOps,
		Splitter:        paneOps,
		PasteSender:     paneOps,
		ServerName:      "test-server",
		ServerVersion:   "test-version",
	})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if rt.store != nil {
		t.Fatal("did not expect fallback store when all repositories are injected")
	}
}
