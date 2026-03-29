package usecase

import (
	"context"
	"errors"
	"io"
	"log"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

type testAgentRepo struct {
	agents       map[string]domain.Agent
	upsertErr    error
	getAgentErr  error
	getByPaneErr error
	listErr      error
}

func newTestAgentRepo() *testAgentRepo {
	return &testAgentRepo{agents: make(map[string]domain.Agent)}
}

func (r *testAgentRepo) UpsertAgent(_ context.Context, agent domain.Agent) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.agents[agent.Name] = agent
	return nil
}

func (r *testAgentRepo) GetAgent(_ context.Context, name string) (domain.Agent, error) {
	if r.getAgentErr != nil {
		return domain.Agent{}, r.getAgentErr
	}
	agent, ok := r.agents[name]
	if !ok {
		return domain.Agent{}, domain.ErrNotFound
	}
	return agent, nil
}

func (r *testAgentRepo) GetAgentByPaneID(_ context.Context, paneID string) (domain.Agent, error) {
	if r.getByPaneErr != nil {
		return domain.Agent{}, r.getByPaneErr
	}
	for _, agent := range r.agents {
		if agent.PaneID == paneID {
			return agent, nil
		}
	}
	return domain.Agent{}, domain.ErrNotFound
}

func (r *testAgentRepo) ListAgents(_ context.Context) ([]domain.Agent, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	agents := make([]domain.Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents, nil
}

func (r *testAgentRepo) DeleteAgentsByPaneID(_ context.Context, paneID string) error {
	for name, agent := range r.agents {
		if agent.PaneID == paneID {
			delete(r.agents, name)
		}
	}
	return nil
}

type testTaskRepo struct {
	tasks           map[string]domain.Task
	createErr       error
	getTaskErr      error
	listErr         error
	completeTaskErr error
	markFailedErr   error
}

func newTestTaskRepo() *testTaskRepo {
	return &testTaskRepo{tasks: make(map[string]domain.Task)}
}

func (r *testTaskRepo) CreateTask(_ context.Context, task domain.Task) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.tasks[task.ID] = task
	return nil
}

func (r *testTaskRepo) GetTask(_ context.Context, taskID string) (domain.Task, error) {
	if r.getTaskErr != nil {
		return domain.Task{}, r.getTaskErr
	}
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.Task{}, domain.ErrNotFound
	}
	return task, nil
}

func (r *testTaskRepo) ListTasks(_ context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	tasks := make([]domain.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if filter.Status != "" && filter.Status != "all" && task.Status != filter.Status {
			continue
		}
		if filter.AgentName != "" && task.AgentName != filter.AgentName {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *testTaskRepo) CompleteTask(_ context.Context, taskID string, responseID string, completedAt string) error {
	if r.completeTaskErr != nil {
		return r.completeTaskErr
	}
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	if task.Status != "pending" {
		return errors.New("task is not pending")
	}
	task.Status = "completed"
	task.SendResponseID = responseID
	task.CompletedAt = completedAt
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) MarkTaskFailed(_ context.Context, taskID string) error {
	if r.markFailedErr != nil {
		return r.markFailedErr
	}
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	task.Status = "failed"
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) AbandonTasksByPaneID(context.Context, string) error {
	return nil
}

func (r *testTaskRepo) EndSessionByInstanceID(context.Context, string) error {
	return nil
}

func (r *testTaskRepo) GetTaskBySendMessageID(_ context.Context, sendMessageID string) (domain.Task, error) {
	if r.getTaskErr != nil {
		return domain.Task{}, r.getTaskErr
	}
	for _, task := range r.tasks {
		if task.SendMessageID == sendMessageID {
			return task, nil
		}
	}
	return domain.Task{}, domain.ErrNotFound
}

type testMessageRepo struct {
	messages  map[string]domain.TaskMessage
	responses map[string]domain.TaskMessage
	saveErr   error
}

func newTestMessageRepo() *testMessageRepo {
	return &testMessageRepo{
		messages:  make(map[string]domain.TaskMessage),
		responses: make(map[string]domain.TaskMessage),
	}
}

func (r *testMessageRepo) SaveMessage(_ context.Context, msg domain.TaskMessage) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.messages[msg.ID] = msg
	return nil
}

func (r *testMessageRepo) SaveResponse(_ context.Context, msg domain.TaskMessage) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.responses[msg.ID] = msg
	return nil
}

func (r *testMessageRepo) GetMessage(_ context.Context, id string) (domain.TaskMessage, error) {
	msg, ok := r.messages[id]
	if !ok {
		return domain.TaskMessage{}, domain.ErrNotFound
	}
	return msg, nil
}

type testPaneOps struct {
	selfPane   string
	sendErr    error
	captureErr error
	sent       []sentCall
}

type sentCall struct {
	paneID string
	text   string
}

func (p *testPaneOps) SendKeys(_ context.Context, paneID string, text string) error {
	if p.sendErr != nil {
		return p.sendErr
	}
	p.sent = append(p.sent, sentCall{paneID: paneID, text: text})
	return nil
}

func (p *testPaneOps) GetPaneID(context.Context) (string, error) {
	return p.selfPane, nil
}

func (p *testPaneOps) ListPanes(context.Context) ([]domain.PaneInfo, error) {
	return nil, nil
}

func (p *testPaneOps) SetPaneTitle(context.Context, string, string) error {
	return nil
}

func (p *testPaneOps) CapturePaneOutput(context.Context, string, int) (string, error) {
	if p.captureErr != nil {
		return "", p.captureErr
	}
	return "captured", nil
}

func discardLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func TestConstructorsDefaultToStandardLogger(t *testing.T) {
	agents := newTestAgentRepo()
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{}

	tests := []struct {
		name string
		got  *log.Logger
	}{
		{"agent", NewAgentService(agents, panes, panes, panes, nil).logger},
		{"dispatch", NewTaskDispatchService(agents, tasks, messages, panes, nil).logger},
		{"query", NewTaskQueryService(agents, tasks, messages, panes, nil).logger},
		{"response", NewResponseService(agents, tasks, messages, panes, panes, nil).logger},
		{"capture", NewCaptureService(agents, panes, panes, nil).logger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got == nil {
				t.Fatal("logger should default to log.Default()")
			}
		})
	}
}

func TestResolveCallerReturnsTrustedOnResolverError(t *testing.T) {
	repo := newTestAgentRepo()
	logger := discardLogger()

	// When the resolver fails (e.g. no TMUX_PANE in pipe bridge mode),
	// resolveCaller returns a trusted caller instead of an error.
	agent, err := resolveCaller(context.Background(), errorResolver{err: errors.New("tmux missing")}, repo, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !IsTrustedCaller(agent) {
		t.Fatalf("expected trusted caller, got: %+v", agent)
	}
}

func TestResolveCallerReturnsTrustedOnEmptyPane(t *testing.T) {
	repo := newTestAgentRepo()
	logger := discardLogger()
	panes := &testPaneOps{selfPane: ""}

	agent, err := resolveCaller(context.Background(), panes, repo, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !IsTrustedCaller(agent) {
		t.Fatalf("expected trusted caller, got: %+v", agent)
	}
}

type errorResolver struct {
	err error
}

func (r errorResolver) GetPaneID(context.Context) (string, error) {
	return "", r.err
}

func TestAgentServiceRegisterPropagatesUpsertFailure(t *testing.T) {
	repo := newTestAgentRepo()
	repo.upsertErr = errors.New("db busy")
	panes := &testPaneOps{}
	svc := NewAgentService(repo, panes, panes, panes, discardLogger())

	_, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "codex", PaneID: "%1"})
	if err == nil || err.Error() != "failed to register agent" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentServiceRegisterAllowsReservedNameUpdate(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewAgentService(repo, panes, panes, panes, discardLogger())

	result, err := svc.Register(context.Background(), RegisterAgentCmd{
		Name:   "orchestrator",
		PaneID: "%1",
		Role:   "controller",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "orchestrator" || result.PaneID != "%1" || result.PaneTitle != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if repo.agents["orchestrator"].PaneID != "%1" {
		t.Fatalf("agent pane_id = %q", repo.agents["orchestrator"].PaneID)
	}
}

func TestAgentServiceRegisterReplacesExistingPaneRegistration(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["old"] = domain.Agent{Name: "old", PaneID: "%2"}
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewAgentService(repo, panes, panes, panes, discardLogger())

	result, err := svc.Register(context.Background(), RegisterAgentCmd{
		Name:   "new",
		PaneID: "%2",
		Role:   "updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "new" || result.PaneID != "%2" || result.Role != "updated" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, ok := repo.agents["old"]; ok {
		t.Fatalf("old registration should be removed: %+v", repo.agents)
	}
	if repo.agents["new"].PaneID != "%2" {
		t.Fatalf("agent pane_id = %q", repo.agents["new"].PaneID)
	}
}

func TestTaskDispatchServiceSendReturnsPersistError(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.createErr = errors.New("db busy")
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName: "codex",
		FromAgent: "worker",
		Message:   "implement it",
	})
	if err == nil || err.Error() != "failed to persist task" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(panes.sent) != 0 {
		t.Fatalf("message should not be sent when persistence fails: %+v", panes.sent)
	}
}

func TestTaskDispatchServiceSendReturnsIDGenerationError(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, discardLogger())
	svc.randRead = func([]byte) (int, error) {
		return 0, errors.New("rng down")
	}

	_, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName: "codex",
		FromAgent: "worker",
		Message:   "implement it",
	})
	if err == nil || err.Error() != "failed to generate task id" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskDispatchServiceSendAllowsUnregisteredCallerWhenSenderExists(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName:                   "codex",
		FromAgent:                   "worker",
		Message:                     "implement it",
		IncludeResponseInstructions: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AgentName != "codex" || result.PaneID != "%1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(panes.sent) != 1 || panes.sent[0].paneID != "%1" {
		t.Fatalf("unexpected sent calls: %+v", panes.sent)
	}
	if !strings.Contains(panes.sent[0].text, "task_id="+result.TaskID) {
		t.Fatalf("sent message should contain concrete task_id: %q", panes.sent[0].text)
	}
	if !strings.Contains(panes.sent[0].text, "send_response(task_id=\""+result.TaskID+"\", message=\"...\")") {
		t.Fatalf("sent message should contain send_response example: %q", panes.sent[0].text)
	}
	task, ok := tasks.tasks[result.TaskID]
	if !ok {
		t.Fatalf("task %s should be persisted", result.TaskID)
	}
	if task.AssigneePaneID != "%1" {
		t.Fatalf("assignee pane = %q, want %%1", task.AssigneePaneID)
	}
	if task.SendMessageID == "" {
		t.Fatal("send_message_id should be set")
	}
	if len(messages.messages) != 1 {
		t.Fatalf("expected 1 message saved, got %d", len(messages.messages))
	}
}

func TestTaskDispatchServiceSendRejectsUnknownSender(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName: "codex",
		FromAgent: "worker",
		Message:   "implement it",
	})
	if err == nil || err.Error() != "sender agent is not available" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks.tasks) != 0 {
		t.Fatalf("tasks should remain empty: %+v", tasks.tasks)
	}
}

func TestResponseServiceSendReturnsNotAvailableForUnknownTask(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err == nil || err.Error() != "task is not available" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponseServiceSendRejectsNonPendingTask(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "worker", SenderName: "codex", Status: "completed"}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err == nil || err.Error() != "task is not pending" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponseServiceSendRejectsMissingSender(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{ID: "t-001", AgentName: "worker", Status: "pending"}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err == nil || err.Error() != "task sender is unknown; cannot deliver response" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponseServiceSendAllowsSamePaneAfterAgentRename(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["renamed"] = domain.Agent{Name: "renamed", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if tasks.tasks["t-001"].Status != "completed" {
		t.Fatalf("task status = %s, want completed", tasks.tasks["t-001"].Status)
	}
	if len(messages.responses) != 1 {
		t.Fatalf("expected 1 response saved, got %d", len(messages.responses))
	}
}

func TestResponseServiceSendAllowsAgentNameFallbackAfterPaneMove(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResponseServiceSendRejectsMismatchedPaneAndAgentName(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["other"] = domain.Agent{Name: "other", PaneID: "%2"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err == nil || err.Error() != "access denied" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResponseServiceSendAllowsLegacyTaskWithoutAssigneePaneID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:         "t-001",
		AgentName:  "worker",
		SenderName: "orchestrator",
		Status:     "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResponseServiceSendAllowsTrustedCaller(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	svc := NewResponseService(agents, tasks, messages, &testPaneOps{}, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResponseServiceSendReturnsWarningOnIDGenerationError(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())
	svc.randRead = func([]byte) (int, error) {
		return 0, errors.New("rng down")
	}

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error (should be best-effort): %v", err)
	}
	if result.Warning == "" {
		t.Fatal("expected warning for ID generation failure")
	}
	if !strings.Contains(result.Warning, "response id generation failed") {
		t.Fatalf("unexpected warning: %q", result.Warning)
	}
	if result.TaskID != "t-001" {
		t.Fatalf("TaskID = %q, want %q", result.TaskID, "t-001")
	}
	// Message should still be delivered despite ID generation failure.
	if len(panes.sent) != 1 || panes.sent[0].paneID != "%0" {
		t.Fatalf("unexpected sent calls: %+v", panes.sent)
	}
}

func TestResponseServiceSendSkipsSendKeysForVirtualPane(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["task-master"] = domain.Agent{Name: "task-master", PaneID: "%virtual-task-master"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "task-master",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "task-master" {
		t.Fatalf("expected SentToName=task-master, got %q", result.SentToName)
	}
	// SendKeys should NOT be called for virtual pane.
	if len(panes.sent) != 0 {
		t.Fatalf("SendKeys should not be called for virtual pane, got %d calls", len(panes.sent))
	}
}

func TestResponseServiceSendCallsSendKeysForRealPane(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("expected SentToName=orchestrator, got %q", result.SentToName)
	}
	// SendKeys SHOULD be called for real pane.
	if len(panes.sent) != 1 || panes.sent[0].paneID != "%0" {
		t.Fatalf("expected SendKeys to %q, got %+v", "%0", panes.sent)
	}
}

func TestTaskDispatchServiceSendRejectsVirtualPane(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["task-master"] = domain.Agent{Name: "task-master", PaneID: "%virtual-task-master"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName: "task-master",
		FromAgent: "worker",
		Message:   "do something",
	})
	if err == nil {
		t.Fatal("expected error when sending to virtual pane agent")
	}
	if err.Error() != "cannot send task to virtual pane agent" {
		t.Fatalf("unexpected error: %v", err)
	}
	// No task should be created in the repo.
	if len(tasks.tasks) != 0 {
		t.Fatalf("tasks should remain empty: %+v", tasks.tasks)
	}
}

func TestCaptureServiceCaptureReturnsWarningForVirtualPane(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["task-master"] = domain.Agent{Name: "task-master", PaneID: "%virtual-task-master"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewCaptureService(agents, panes, panes, discardLogger())

	result, err := svc.Capture(context.Background(), CapturePaneCmd{AgentName: "task-master", Lines: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Warning != "virtual pane cannot be captured" {
		t.Fatalf("expected warning about virtual pane, got %q", result.Warning)
	}
	if result.Content != "" {
		t.Fatalf("expected empty content for virtual pane, got %q", result.Content)
	}
}

func TestTaskQueryServiceCheckTasksCountsAllStatuses(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", Status: "pending"}
	tasks.tasks["t-2"] = domain.Task{ID: "t-2", AgentName: "worker", Status: "completed"}
	tasks.tasks["t-3"] = domain.Task{ID: "t-3", AgentName: "worker", Status: "failed"}
	tasks.tasks["t-4"] = domain.Task{ID: "t-4", AgentName: "worker", Status: "abandoned"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, discardLogger())

	result, err := svc.CheckTasks(context.Background(), CheckTasksCmd{StatusFilter: "all"})
	if err != nil {
		t.Fatalf("CheckTasks: %v", err)
	}

	if !reflect.DeepEqual([]int{result.Pending, result.Completed, result.Failed, result.Abandoned}, []int{1, 1, 1, 1}) {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestTaskQueryServiceGetMyTasksIncludesTaskIDPlaceholderInstruction(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", Status: "pending"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, discardLogger())

	result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("GetMyTasks: %v", err)
	}
	if !strings.Contains(result.ResponseInstructions, "task_id=<task_id>") {
		t.Fatalf("response instructions should include placeholder task_id: %q", result.ResponseInstructions)
	}
	if !strings.Contains(result.ResponseInstructions, "send_response(task_id=\"<task_id>\", message=\"...\")") {
		t.Fatalf("response instructions should include send_response example: %q", result.ResponseInstructions)
	}
}

func TestTaskQueryServiceGetMyTask(t *testing.T) {
	tests := []struct {
		name        string
		callerPane  string
		agentName   string
		msgID       string
		setupAgent  map[string]domain.Agent
		setupTask   map[string]domain.Task
		setupMsg    map[string]domain.TaskMessage
		wantErr     string
		wantTaskID  string
		wantContent string
	}{
		{
			name:       "success",
			callerPane: "%1",
			agentName:  "worker",
			msgID:      "m-001",
			setupAgent: map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", SendMessageID: "m-001", Status: "pending", SentAt: "2026-03-07T10:00:00Z"}},
			setupMsg:   map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "hello world", CreatedAt: "2026-03-07T10:00:00Z"}},
			wantTaskID: "t-001", wantContent: "hello world",
		},
		{
			name:       "access denied - caller mismatch",
			callerPane: "%2",
			agentName:  "worker",
			msgID:      "m-001",
			setupAgent: map[string]domain.Agent{"other": {Name: "other", PaneID: "%2"}, "worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", SendMessageID: "m-001", Status: "pending"}},
			setupMsg:   map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "hello"}},
			wantErr:    "access denied",
		},
		{
			name:       "access denied - task belongs to other agent",
			callerPane: "%1",
			agentName:  "worker",
			msgID:      "m-001",
			setupAgent: map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "other", SendMessageID: "m-001", Status: "pending"}},
			setupMsg:   map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "hello"}},
			wantErr:    "access denied",
		},
		{
			name:       "task not found",
			callerPane: "%1",
			agentName:  "worker",
			msgID:      "m-nonexistent",
			setupAgent: map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{},
			setupMsg:   map[string]domain.TaskMessage{},
			wantErr:    "task not found",
		},
		{
			name:       "message not found",
			callerPane: "%1",
			agentName:  "worker",
			msgID:      "m-001",
			setupAgent: map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", SendMessageID: "m-001", Status: "pending"}},
			setupMsg:   map[string]domain.TaskMessage{},
			wantErr:    "message not found",
		},
		{
			name:       "trusted caller bypasses pane check",
			callerPane: "",
			agentName:  "worker",
			msgID:      "m-001",
			setupAgent: map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:  map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", SendMessageID: "m-001", Status: "pending", SentAt: "2026-03-07T10:00:00Z"}},
			setupMsg:   map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "trusted", CreatedAt: "2026-03-07T10:00:00Z"}},
			wantTaskID: "t-001", wantContent: "trusted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := &testAgentRepo{agents: tt.setupAgent}
			tasks := &testTaskRepo{tasks: tt.setupTask}
			messages := &testMessageRepo{messages: tt.setupMsg, responses: make(map[string]domain.TaskMessage)}
			panes := &testPaneOps{selfPane: tt.callerPane}
			svc := NewTaskQueryService(agents, tasks, messages, panes, discardLogger())

			result, err := svc.GetMyTask(context.Background(), GetMyTaskCmd{
				AgentName:     tt.agentName,
				SendMessageID: tt.msgID,
			})

			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TaskID != tt.wantTaskID {
				t.Errorf("TaskID = %q, want %q", result.TaskID, tt.wantTaskID)
			}
			if result.Message.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Message.Content, tt.wantContent)
			}
			if result.SendMessageID != tt.msgID {
				t.Errorf("SendMessageID = %q, want %q", result.SendMessageID, tt.msgID)
			}
		})
	}
}

func TestTaskQueryServiceGetMyTasksIncludesSendMessageID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", SendMessageID: "m-001", Status: "pending"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, discardLogger())

	result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{AgentName: "worker", StatusFilter: "pending"})
	if err != nil {
		t.Fatalf("GetMyTasks: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(result.Tasks))
	}
	if result.Tasks[0].SendMessageID != "m-001" {
		t.Errorf("SendMessageID = %q, want %q", result.Tasks[0].SendMessageID, "m-001")
	}
}

func TestTaskQueryServiceCheckTasksIncludesSendMessageID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", SendMessageID: "m-002", Status: "pending"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, discardLogger())

	result, err := svc.CheckTasks(context.Background(), CheckTasksCmd{StatusFilter: "all"})
	if err != nil {
		t.Fatalf("CheckTasks: %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(result.Tasks))
	}
	if result.Tasks[0].SendMessageID != "m-002" {
		t.Errorf("SendMessageID = %q, want %q", result.Tasks[0].SendMessageID, "m-002")
	}
}

// ---------------------------------------------------------------------------
// generateIDWith direct tests
// ---------------------------------------------------------------------------

func TestGenerateIDWith(t *testing.T) {
	t.Parallel()

	fixedBytes := []byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}
	fixedRand := func(b []byte) (int, error) {
		return copy(b, fixedBytes), nil
	}

	tests := []struct {
		name    string
		readFn  func([]byte) (int, error)
		prefix  string
		wantID  string
		wantErr bool
	}{
		{
			name:   "success with t- prefix",
			readFn: fixedRand,
			prefix: "t-",
			wantID: "t-deadbeefcafe",
		},
		{
			name:   "success with r- prefix",
			readFn: fixedRand,
			prefix: "r-",
			wantID: "r-deadbeefcafe",
		},
		{
			name:   "success with m- prefix",
			readFn: fixedRand,
			prefix: "m-",
			wantID: "m-deadbeefcafe",
		},
		{
			name:    "readFn error",
			readFn:  func([]byte) (int, error) { return 0, errors.New("rng down") },
			prefix:  "t-",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := generateIDWith(tt.readFn, tt.prefix, "test context")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id != tt.wantID {
				t.Fatalf("id = %q, want %q", id, tt.wantID)
			}
			// Verify format: prefix + 12 hex characters (6 bytes).
			if !strings.HasPrefix(id, tt.prefix) {
				t.Fatalf("id %q does not start with prefix %q", id, tt.prefix)
			}
			hexPart := id[len(tt.prefix):]
			if len(hexPart) != 12 {
				t.Fatalf("hex part length = %d, want 12", len(hexPart))
			}
		})
	}
}
