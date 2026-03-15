package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"log"
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

func (r *testTaskRepo) CreateTask(context.Context, domain.Task) error { return nil }
func (r *testTaskRepo) GetTask(context.Context, string) (domain.Task, error) {
	return domain.Task{}, domain.ErrNotFound
}
func (r *testTaskRepo) ListTasks(context.Context, domain.TaskFilter) ([]domain.Task, error) {
	return nil, nil
}
func (r *testTaskRepo) CompleteTask(context.Context, string, string, string) error { return nil }
func (r *testTaskRepo) MarkTaskFailed(context.Context, string) error               { return nil }
func (r *testTaskRepo) AbandonTasksByPaneID(_ context.Context, paneID string) error {
	r.abandonCalls = append(r.abandonCalls, paneID)
	return nil
}
func (r *testTaskRepo) EndSessionByInstanceID(_ context.Context, instanceID string) error {
	r.endSessionCalls = append(r.endSessionCalls, instanceID)
	return nil
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
