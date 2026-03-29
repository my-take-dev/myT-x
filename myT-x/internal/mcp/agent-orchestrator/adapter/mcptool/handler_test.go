package mcptool

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/mcp/agent-orchestrator/usecase"
)

type mockRepo struct {
	agents            map[string]domain.Agent
	tasks             map[string]domain.Task
	completeTaskErr   error
	markTaskFailedErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		agents: make(map[string]domain.Agent),
		tasks:  make(map[string]domain.Task),
	}
}

func (m *mockRepo) UpsertAgent(_ context.Context, a domain.Agent) error {
	m.agents[a.Name] = a
	return nil
}

func (m *mockRepo) GetAgent(_ context.Context, name string) (domain.Agent, error) {
	a, ok := m.agents[name]
	if !ok {
		return domain.Agent{}, fmt.Errorf("agent %q not found: %w", name, domain.ErrNotFound)
	}
	return a, nil
}

func (m *mockRepo) GetAgentByPaneID(_ context.Context, paneID string) (domain.Agent, error) {
	for _, agent := range m.agents {
		if agent.PaneID == paneID {
			return agent, nil
		}
	}
	return domain.Agent{}, fmt.Errorf("pane %q not found: %w", paneID, domain.ErrNotFound)
}

func (m *mockRepo) ListAgents(_ context.Context) ([]domain.Agent, error) {
	agents := make([]domain.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		agents = append(agents, a)
	}
	return agents, nil
}

func (m *mockRepo) DeleteAgentsByPaneID(_ context.Context, paneID string) error {
	for name, a := range m.agents {
		if a.PaneID == paneID {
			delete(m.agents, name)
		}
	}
	return nil
}

func (m *mockRepo) CreateTask(_ context.Context, t domain.Task) error {
	m.tasks[t.ID] = t
	return nil
}

func (m *mockRepo) GetTask(_ context.Context, taskID string) (domain.Task, error) {
	t, ok := m.tasks[taskID]
	if !ok {
		return domain.Task{}, fmt.Errorf("task %q: %w", taskID, domain.ErrNotFound)
	}
	return t, nil
}

func (m *mockRepo) ListTasks(_ context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	var result []domain.Task
	for _, t := range m.tasks {
		if filter.Status != "" && filter.Status != "all" && t.Status != filter.Status {
			continue
		}
		if filter.AgentName != "" && t.AgentName != filter.AgentName {
			continue
		}
		result = append(result, t)
	}
	return result, nil
}

func (m *mockRepo) CompleteTask(_ context.Context, taskID string, responseID string, completedAt string) error {
	if m.completeTaskErr != nil {
		return m.completeTaskErr
	}
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.Status != "pending" {
		return fmt.Errorf("task %q is %s", taskID, t.Status)
	}
	t.Status = "completed"
	t.SendResponseID = responseID
	t.CompletedAt = completedAt
	m.tasks[taskID] = t
	return nil
}

func (m *mockRepo) MarkTaskFailed(_ context.Context, taskID string) error {
	if m.markTaskFailedErr != nil {
		return m.markTaskFailedErr
	}
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %q not found", taskID)
	}
	if t.Status != "pending" {
		return fmt.Errorf("task %q is %s", taskID, t.Status)
	}
	t.Status = "failed"
	m.tasks[taskID] = t
	return nil
}

func (m *mockRepo) AbandonTasksByPaneID(_ context.Context, paneID string) error {
	targetAgents := make(map[string]struct{})
	for _, a := range m.agents {
		if a.PaneID == paneID {
			targetAgents[a.Name] = struct{}{}
		}
	}
	for id, t := range m.tasks {
		if t.Status != "pending" {
			continue
		}
		if _, ok := targetAgents[t.AgentName]; ok {
			t.Status = "abandoned"
			m.tasks[id] = t
		}
	}
	return nil
}

func (m *mockRepo) EndSessionByInstanceID(_ context.Context, instanceID string) error {
	return nil
}

func (m *mockRepo) GetTaskBySendMessageID(_ context.Context, sendMessageID string) (domain.Task, error) {
	for _, t := range m.tasks {
		if t.SendMessageID == sendMessageID {
			return t, nil
		}
	}
	return domain.Task{}, fmt.Errorf("task by send_message_id %q: %w", sendMessageID, domain.ErrNotFound)
}

type mockMessageRepo struct {
	messages  map[string]domain.TaskMessage
	responses map[string]domain.TaskMessage
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{
		messages:  make(map[string]domain.TaskMessage),
		responses: make(map[string]domain.TaskMessage),
	}
}

func (m *mockMessageRepo) SaveMessage(_ context.Context, msg domain.TaskMessage) error {
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockMessageRepo) SaveResponse(_ context.Context, msg domain.TaskMessage) error {
	m.responses[msg.ID] = msg
	return nil
}

func (m *mockMessageRepo) GetMessage(_ context.Context, id string) (domain.TaskMessage, error) {
	msg, ok := m.messages[id]
	if !ok {
		return domain.TaskMessage{}, fmt.Errorf("message %q: %w", id, domain.ErrNotFound)
	}
	return msg, nil
}

type mockPaneOps struct {
	selfPane   string
	sentKeys   []sentKey
	paneTitle  map[string]string
	capturedAt map[string]string
	sendErr    error
	captureErr error
	listErr    error
}

type sentKey struct {
	paneID string
	text   string
}

func newMockPaneOps() *mockPaneOps {
	return &mockPaneOps{
		selfPane:   "%0",
		paneTitle:  make(map[string]string),
		capturedAt: make(map[string]string),
	}
}

func (m *mockPaneOps) SendKeys(_ context.Context, paneID string, text string) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentKeys = append(m.sentKeys, sentKey{paneID: paneID, text: text})
	return nil
}

func (m *mockPaneOps) GetPaneID(context.Context) (string, error) {
	return m.selfPane, nil
}

func (m *mockPaneOps) ListPanes(context.Context) ([]domain.PaneInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return []domain.PaneInfo{
		{ID: "%0", Title: "orchestrator"},
		{ID: "%1", Title: "codex"},
		{ID: "%2", Title: ""},
	}, nil
}

func (m *mockPaneOps) SetPaneTitle(_ context.Context, paneID string, title string) error {
	m.paneTitle[paneID] = title
	return nil
}

func (m *mockPaneOps) CapturePaneOutput(_ context.Context, paneID string, _ int) (string, error) {
	if m.captureErr != nil {
		return "", m.captureErr
	}
	content, ok := m.capturedAt[paneID]
	if !ok {
		return "$ idle\n", nil
	}
	return content, nil
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func buildTestHandler(repo *mockRepo, paneOps *mockPaneOps) *Handler {
	logger := testLogger()
	msgRepo := newMockMessageRepo()
	agentSvc := usecase.NewAgentService(repo, paneOps, paneOps, paneOps, logger)
	dispatchSvc := usecase.NewTaskDispatchService(repo, repo, msgRepo, paneOps, logger)
	querySvc := usecase.NewTaskQueryService(repo, repo, msgRepo, paneOps, logger)
	responseSvc := usecase.NewResponseService(repo, repo, msgRepo, paneOps, paneOps, logger)
	captureSvc := usecase.NewCaptureService(repo, paneOps, paneOps, logger)
	return NewHandler(agentSvc, dispatchSvc, querySvc, responseSvc, captureSvc, nil, "mcp-test-instance")
}

func TestRegisterAgentRequiresSelfPane(t *testing.T) {
	mr := newMockRepo()
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, err := h.BuildRegistry()
	if err != nil {
		t.Fatalf("BuildRegistry: %v", err)
	}
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "codex",
		"pane_id": "%1",
		"role":    "バックエンド実装",
		"skills":  []any{"Goのバックエンド実装とAPI設計を担当", "入出力バリデーションとエラーハンドリングを確認"},
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	if m["name"] != "codex" || m["pane_id"] != "%1" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if mp.paneTitle["%1"] != "codex:バックエンド実装" {
		t.Fatalf("pane title = %q", mp.paneTitle["%1"])
	}
}

func TestRegisterAgentAllowsOtherPane(t *testing.T) {
	mr := newMockRepo()
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "codex",
		"pane_id": "%2",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["pane_id"] != "%2" {
		t.Fatalf("unexpected pane_id: %v", m["pane_id"])
	}
}

func TestRegisterAgentAllowsReservedOverwrite(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "orchestrator",
		"pane_id": "%1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["name"] != "orchestrator" || m["pane_id"] != "%1" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if mp.paneTitle["%1"] != "orchestrator" {
		t.Fatalf("pane title = %q", mp.paneTitle["%1"])
	}
}

func TestRegisterAgentAllowsUpdate(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "codex",
		"pane_id": "%1",
		"role":    "更新されたロール",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["role"] != "更新されたロール" {
		t.Fatalf("unexpected role: %v", m["role"])
	}
}

func TestRegisterAgentAllowsUpdateFromOtherPane(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1", Role: "old"}
	mp := newMockPaneOps()
	mp.selfPane = "%9"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "codex",
		"pane_id": "%2",
		"role":    "updated",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["name"] != "codex" || m["pane_id"] != "%2" || m["role"] != "updated" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if _, ok := mr.agents["codex"]; !ok {
		t.Fatalf("updated registration should exist: %+v", mr.agents)
	}
	if mr.agents["codex"].PaneID != "%2" {
		t.Fatalf("agent pane_id = %q", mr.agents["codex"].PaneID)
	}
	if mp.paneTitle["%2"] != "codex:updated" {
		t.Fatalf("pane title = %q", mp.paneTitle["%2"])
	}
}

func TestRegisterAgentReplacesExistingPaneRegistration(t *testing.T) {
	mr := newMockRepo()
	mr.agents["old-agent"] = domain.Agent{Name: "old-agent", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%9"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("register_agent")

	result, err := tool.Handler(context.Background(), map[string]any{
		"name":    "new-agent",
		"pane_id": "%1",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["name"] != "new-agent" || m["pane_id"] != "%1" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if _, ok := mr.agents["old-agent"]; ok {
		t.Fatalf("old pane registration should be removed: %+v", mr.agents)
	}
	if _, ok := mr.agents["new-agent"]; !ok {
		t.Fatalf("new pane registration should exist: %+v", mr.agents)
	}
}

func TestSendTaskAllowsAnyRegisteredAgent(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	mp := newMockPaneOps()
	mp.selfPane = "%9"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_task")

	result, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "codex",
		"from_agent": "worker",
		"message":    "hello",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["agent_name"] != "codex" {
		t.Fatalf("unexpected agent_name: %v", m["agent_name"])
	}
	if _, err := requiredTaskID(map[string]any{"task_id": m["task_id"]}, "task_id"); err != nil {
		t.Fatalf("task_id should satisfy validation: %v", err)
	}
	taskID, _ := m["task_id"].(string)
	if mr.tasks[taskID].AssigneePaneID != "%1" {
		t.Fatalf("assignee pane = %q, want %%1", mr.tasks[taskID].AssigneePaneID)
	}
}

func TestSendTaskRejectsUnknownSender(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%9"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_task")

	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "codex",
		"from_agent": "unknown",
		"message":    "hello",
	})
	if err == nil || err.Error() != "sender agent is not available" {
		t.Fatalf("expected sender agent is not available, got %v", err)
	}
}

func TestSendTaskPersistsBeforeDeliveryAndMarksFailure(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%0"
	mp.sendErr = fmt.Errorf("tmux failed")

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_task")

	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "codex",
		"from_agent": "orchestrator",
		"message":    "APIを実装してください",
	})
	if err == nil || err.Error() != "message delivery failed" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mr.tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(mr.tasks))
	}
	for _, task := range mr.tasks {
		if task.Status != "failed" {
			t.Fatalf("task status = %s, want failed", task.Status)
		}
	}
}

func TestSendTaskWarnsWhenFailureStatusUpdateFails(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.markTaskFailedErr = fmt.Errorf("sqlite busy")
	mp := newMockPaneOps()
	mp.selfPane = "%0"
	mp.sendErr = fmt.Errorf("tmux failed")

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_task")

	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "codex",
		"from_agent": "orchestrator",
		"message":    "APIを実装してください",
	})
	if err == nil || err.Error() != "message delivery failed; task may remain pending" {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, task := range mr.tasks {
		if task.Status != "pending" {
			t.Fatalf("task status = %s, want pending", task.Status)
		}
	}
}

func TestGetMyTasksRequiresCallerMatch(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("get_my_tasks")

	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "other",
	})
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestSendResponseRequiresTaskOwnership(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	mr.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "codex", SenderName: "orchestrator", Status: "pending"}
	mp := newMockPaneOps()
	mp.selfPane = "%2"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_response")

	_, err := tool.Handler(context.Background(), map[string]any{
		"message": "done",
		"task_id": "t-001",
	})
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("expected access denied, got %v", err)
	}
}

func TestSendResponseCompletesTask(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "codex", AssigneePaneID: "%1", SenderName: "orchestrator", Status: "pending"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_response")

	result, err := tool.Handler(context.Background(), map[string]any{
		"message": "実装完了しました",
		"task_id": "t-001",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	if m["sent_to_name"] != "orchestrator" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if mr.tasks["t-001"].Status != "completed" {
		t.Fatalf("task status = %s", mr.tasks["t-001"].Status)
	}
	if mr.tasks["t-001"].CompletedAt == "" {
		t.Fatal("completed_at should be set")
	}
}

func TestSendResponseAllowsSamePaneAfterRename(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["renamed"] = domain.Agent{Name: "renamed", PaneID: "%1"}
	mr.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "codex", AssigneePaneID: "%1", SenderName: "orchestrator", Status: "pending"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_response")

	result, err := tool.Handler(context.Background(), map[string]any{
		"message": "実装完了しました",
		"task_id": "t-001",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	if m["sent_to_name"] != "orchestrator" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if mr.tasks["t-001"].Status != "completed" {
		t.Fatalf("task status = %s", mr.tasks["t-001"].Status)
	}
}

func TestSendResponseReturnsWarningWhenTaskCompletionUpdateFails(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "codex", SenderName: "orchestrator", Status: "pending"}
	mr.completeTaskErr = fmt.Errorf("sqlite busy")
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_response")

	result, err := tool.Handler(context.Background(), map[string]any{
		"message": "実装完了しました",
		"task_id": "t-001",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	if m["warning"] != "message delivered but task completion update failed" {
		t.Fatalf("unexpected result: %+v", m)
	}
	if len(mp.sentKeys) != 1 || mp.sentKeys[0].paneID != "%0" {
		t.Fatalf("unexpected sent keys: %+v", mp.sentKeys)
	}
	if mr.tasks["t-001"].Status != "pending" {
		t.Fatalf("task status = %s, want pending", mr.tasks["t-001"].Status)
	}
}

func TestCheckTasksAllowsRegisteredAgent(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "codex", Status: "pending"}
	mr.tasks["t-2"] = domain.Task{ID: "t-2", AgentName: "codex", Status: "completed"}
	mr.tasks["t-3"] = domain.Task{ID: "t-3", AgentName: "codex", Status: "failed"}
	mr.tasks["t-4"] = domain.Task{ID: "t-4", AgentName: "codex", Status: "abandoned"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("check_tasks")

	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	m := result.(map[string]any)
	if m["tasks"] == nil {
		t.Fatal("expected tasks in result")
	}
	summary, ok := m["summary"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected summary type: %T", m["summary"])
	}
	if summary["pending"] != 1 || summary["completed"] != 1 || summary["failed"] != 1 || summary["abandoned"] != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestCapturePaneAllowsRegisteredAgent(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.capturedAt["%1"] = "$ go test ./...\nok\n"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("capture_pane")

	mp.selfPane = "%1"
	result, err := tool.Handler(context.Background(), map[string]any{"agent_name": "codex"})
	if err != nil {
		t.Fatalf("registered agent capture should succeed: %v", err)
	}
	m := result.(map[string]any)
	if m["content"] != "$ go test ./...\nok\n" {
		t.Fatalf("unexpected content: %v", m["content"])
	}

	mp.selfPane = "%0"
	if _, err := tool.Handler(context.Background(), map[string]any{"agent_name": "codex"}); err != nil {
		t.Fatalf("orchestrator capture should succeed: %v", err)
	}
}

func TestCapturePaneValidatesLines(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("capture_pane")

	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name": "codex",
		"lines":      float64(1000),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCapturePaneReturnsGenericWarningOnCaptureFailure(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"
	mp.captureErr = fmt.Errorf("tmux failed")

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("capture_pane")

	result, err := tool.Handler(context.Background(), map[string]any{"agent_name": "codex"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["warning"] != "pane capture failed" {
		t.Fatalf("unexpected warning: %+v", m)
	}
}

func TestCapturePaneReturnsWarningWhenUnsupported(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"
	mp.captureErr = fmt.Errorf("capture-pane: exit status 1: Access is denied.")

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("capture_pane")

	result, err := tool.Handler(context.Background(), map[string]any{"agent_name": "codex"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := result.(map[string]any)
	if m["warning"] != "pane capture is unavailable on the current tmux shim" {
		t.Fatalf("unexpected warning: %+v", m)
	}
	if m["content"] != "" {
		t.Fatalf("content should be empty when unsupported: %+v", m)
	}
}

func TestListAgentsSeparatesOrchestratorAndUnregisteredPanes(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1", Role: "reviewer"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("list_agents")

	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	orchestrator, ok := m["orchestrator"].(map[string]any)
	if !ok || orchestrator["pane_id"] != "%0" {
		t.Fatalf("unexpected orchestrator: %+v", m["orchestrator"])
	}
	registered, ok := m["registered_agents"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected registered_agents type: %T", m["registered_agents"])
	}
	if len(registered) != 1 || registered[0]["name"] != "codex" {
		t.Fatalf("unexpected registered agents: %+v", registered)
	}
	unregistered, ok := m["unregistered_panes"].([]string)
	if !ok {
		t.Fatalf("unexpected unregistered_panes type: %T", m["unregistered_panes"])
	}
	if len(unregistered) != 1 || unregistered[0] != "%2" {
		t.Fatalf("unexpected unregistered panes: %+v", unregistered)
	}
}

func TestListAgentsReturnsWarningWhenPaneInspectionFails(t *testing.T) {
	mr := newMockRepo()
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"
	mp.listErr = fmt.Errorf("tmux failed")

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("list_agents")

	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	m := result.(map[string]any)
	if m["warning"] != "failed to inspect tmux panes; unregistered_panes may be incomplete" {
		t.Fatalf("unexpected warning: %+v", m)
	}
}

func TestListAgentsReturnsEmptyRegisteredAgentsArray(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mp := newMockPaneOps()
	mp.selfPane = "%0"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("list_agents")

	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	registered, ok := result.(map[string]any)["registered_agents"].([]map[string]any)
	if !ok {
		t.Fatalf("unexpected registered_agents type: %T", result.(map[string]any)["registered_agents"])
	}
	if len(registered) != 0 {
		t.Fatalf("expected empty registered_agents, got %+v", registered)
	}
}

func TestSendResponseRejectsNonPendingTask(t *testing.T) {
	mr := newMockRepo()
	mr.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	mr.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	mr.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "codex", SenderName: "orchestrator", Status: "completed"}
	mp := newMockPaneOps()
	mp.selfPane = "%1"

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()
	tool, _ := registry.Get("send_response")

	_, err := tool.Handler(context.Background(), map[string]any{
		"message": "done",
		"task_id": "t-001",
	})
	if err == nil || err.Error() != "task is not pending" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolsRejectUnregisteredCaller(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		args     map[string]any
	}{
		{name: "get_my_tasks", toolName: "get_my_tasks", args: map[string]any{"agent_name": "worker"}},
		{name: "send_response", toolName: "send_response", args: map[string]any{"message": "done", "task_id": "t-001"}},
		{name: "check_tasks", toolName: "check_tasks", args: map[string]any{}},
		{name: "capture_pane", toolName: "capture_pane", args: map[string]any{"agent_name": "codex"}},
		{name: "list_agents", toolName: "list_agents", args: map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := newMockRepo()
			mp := newMockPaneOps()
			mp.selfPane = "%9"

			h := buildTestHandler(mr, mp)
			registry, _ := h.BuildRegistry()
			tool, _ := registry.Get(tt.toolName)

			_, err := tool.Handler(context.Background(), tt.args)
			if err == nil || err.Error() != "caller is not registered" {
				t.Fatalf("expected caller is not registered, got %v", err)
			}
		})
	}
}

func TestMockRepoAbandonTasksByPaneID(t *testing.T) {
	mr := newMockRepo()
	mr.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	mr.agents["other"] = domain.Agent{Name: "other", PaneID: "%3"}
	mr.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", Status: "pending"}
	mr.tasks["t-2"] = domain.Task{ID: "t-2", AgentName: "worker", Status: "completed"}
	mr.tasks["t-3"] = domain.Task{ID: "t-3", AgentName: "other", Status: "pending"}

	if err := mr.AbandonTasksByPaneID(context.Background(), "%2"); err != nil {
		t.Fatalf("AbandonTasksByPaneID: %v", err)
	}

	if mr.tasks["t-1"].Status != "abandoned" {
		t.Fatalf("t-1 status = %s, want abandoned", mr.tasks["t-1"].Status)
	}
	if mr.tasks["t-2"].Status != "completed" {
		t.Fatalf("t-2 status = %s, want completed", mr.tasks["t-2"].Status)
	}
	if mr.tasks["t-3"].Status != "pending" {
		t.Fatalf("t-3 status = %s, want pending", mr.tasks["t-3"].Status)
	}
}

func TestGetMyTaskHandlerHappyPath(t *testing.T) {
	mr := newMockRepo()
	mp := newMockPaneOps()
	mp.selfPane = "%1"
	mr.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	mr.tasks["t-001"] = domain.Task{
		ID: "t-001", AgentName: "worker", SendMessageID: "m-001",
		Status: "pending", SentAt: "2026-03-07T10:00:00Z",
	}

	h := buildTestHandler(mr, mp)
	registry, _ := h.BuildRegistry()

	// get_my_task の前に send_messages にメッセージを保存する必要がある。
	// buildTestHandler は msgRepo を内部で作るので、直接 get_my_task は message not found になる。
	// ここでは handler 経由のツール呼び出しパターンの検証として、
	// msgRepo が空の場合に適切にエラーが返ることを確認する。
	tool, ok := registry.Get("get_my_task")
	if !ok {
		t.Fatal("get_my_task tool should be registered")
	}
	_, err := tool.Handler(context.Background(), map[string]any{
		"agent_name":      "worker",
		"send_message_id": "m-001",
	})
	// msgRepo にメッセージがないので "message not found" エラーになる
	if err == nil || err.Error() != "message not found" {
		t.Fatalf("expected 'message not found' error, got %v", err)
	}
}

func TestToolCount(t *testing.T) {
	mr := newMockRepo()
	mp := newMockPaneOps()
	h := buildTestHandler(mr, mp)
	registry, err := h.BuildRegistry()
	if err != nil {
		t.Fatalf("BuildRegistry: %v", err)
	}

	tools := registry.List()
	if len(tools) != 10 {
		t.Fatalf("got %d tools, want 10", len(tools))
	}
	for _, name := range []string{
		"register_agent",
		"list_agents",
		"send_task",
		"get_my_tasks",
		"get_my_task",
		"send_response",
		"check_tasks",
		"capture_pane",
		"add_member",
		"help",
	} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("tool %q should be registered", name)
		}
	}
}

func TestHelpOverviewReturnsAllTools(t *testing.T) {
	result, err := handleHelp(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handleHelp: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	tools, ok := m["available_tools"].([]string)
	if !ok {
		t.Fatalf("expected []string for available_tools, got %T", m["available_tools"])
	}
	if len(tools) != 10 {
		t.Fatalf("got %d tools, want 10", len(tools))
	}
}

func TestHelpWithTopicReturnsTool(t *testing.T) {
	result, err := handleHelp(context.Background(), map[string]any{"topic": "send_task"})
	if err != nil {
		t.Fatalf("handleHelp: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["tool"] != "send_task" {
		t.Fatalf("tool = %v, want send_task", m["tool"])
	}
}

func TestHelpWithUnknownTopicReturnsError(t *testing.T) {
	result, err := handleHelp(context.Background(), map[string]any{"topic": "nonexistent"})
	if err != nil {
		t.Fatalf("handleHelp: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if _, hasError := m["error"]; !hasError {
		t.Fatal("expected error field for unknown topic")
	}
	if _, hasTopics := m["available_topics"]; !hasTopics {
		t.Fatal("expected available_topics field for unknown topic")
	}
}

func TestHelpNoAuthRequired(t *testing.T) {
	// help は登録不要で呼べることを確認（paneOps の selfPane を空にしてもエラーにならない）
	mr := newMockRepo()
	mp := newMockPaneOps()
	mp.selfPane = "" // 未登録状態
	h := buildTestHandler(mr, mp)
	registry, err := h.BuildRegistry()
	if err != nil {
		t.Fatalf("BuildRegistry: %v", err)
	}
	tool, ok := registry.Get("help")
	if !ok {
		t.Fatal("help tool should be registered")
	}
	result, err := tool.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("help should not require auth: %v", err)
	}
	if result == nil {
		t.Fatal("help should return content")
	}
}
