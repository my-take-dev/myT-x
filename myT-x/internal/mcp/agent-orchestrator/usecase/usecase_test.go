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

func (r *testTaskRepo) CompleteTask(_ context.Context, taskID string, notes string, completedAt string) error {
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
	task.Notes = notes
	task.CompletedAt = completedAt
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) MarkTaskFailed(_ context.Context, taskID string, notes string) error {
	if r.markFailedErr != nil {
		return r.markFailedErr
	}
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	task.Status = "failed"
	task.Notes = notes
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) AbandonTasksByPaneID(context.Context, string) error {
	return nil
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
	panes := &testPaneOps{}

	tests := []struct {
		name string
		got  *log.Logger
	}{
		{"agent", NewAgentService(agents, panes, panes, panes, nil).logger},
		{"dispatch", NewTaskDispatchService(agents, tasks, panes, nil).logger},
		{"query", NewTaskQueryService(agents, tasks, panes, nil).logger},
		{"response", NewResponseService(agents, tasks, panes, panes, nil).logger},
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
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, panes, discardLogger())

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
	oldRandRead := randRead
	randRead = func([]byte) (int, error) {
		return 0, errors.New("rng down")
	}
	defer func() { randRead = oldRandRead }()

	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewTaskDispatchService(agents, tasks, panes, discardLogger())

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
}

func TestTaskDispatchServiceSendRejectsUnknownSender(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewTaskDispatchService(agents, tasks, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, panes, panes, discardLogger())

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
	svc := NewResponseService(agents, tasks, &testPaneOps{}, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
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
	svc := NewTaskQueryService(agents, tasks, panes, discardLogger())

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
	svc := NewTaskQueryService(agents, tasks, panes, discardLogger())

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

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "hello..."},
		{"日本語テスト", 3, "日本語..."},
		{"line1\nline2", 20, "line1 line2"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := truncate(tt.input, tt.maxLen); got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
