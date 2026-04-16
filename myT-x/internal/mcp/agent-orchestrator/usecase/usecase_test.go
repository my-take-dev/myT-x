package usecase

import (
	"context"
	"errors"
	"io"
	"log"
	"maps"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

type testAgentRepo struct {
	agents          map[string]domain.Agent
	statuses        map[string]domain.AgentStatus
	upsertErr       error
	getAgentErr     error
	getByPaneErr    error
	listErr         error
	statusGetErr    error
	statusListErr   error
	statusUpsertErr error
}

func newTestAgentRepo() *testAgentRepo {
	return &testAgentRepo{
		agents:   make(map[string]domain.Agent),
		statuses: make(map[string]domain.AgentStatus),
	}
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

func (r *testAgentRepo) GetAgentByMCPInstanceID(_ context.Context, instanceID string) (domain.Agent, error) {
	if r.getByPaneErr != nil {
		return domain.Agent{}, r.getByPaneErr
	}

	var matched []domain.Agent
	for _, agent := range r.agents {
		if agent.MCPInstanceID == instanceID {
			matched = append(matched, agent)
		}
	}
	if len(matched) == 0 {
		return domain.Agent{}, domain.ErrNotFound
	}
	if len(matched) > 1 {
		return domain.Agent{}, errors.New("multiple agents registered for instance")
	}
	return matched[0], nil
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
			delete(r.statuses, name)
		}
	}
	return nil
}

func (r *testAgentRepo) ReplaceAgentRegistration(ctx context.Context, agent domain.Agent, defaultStatus *domain.AgentStatus) error {
	agentsSnapshot := maps.Clone(r.agents)
	statusesSnapshot := maps.Clone(r.statuses)
	restore := func() {
		r.agents = agentsSnapshot
		r.statuses = statusesSnapshot
	}

	if err := r.DeleteAgentsByPaneID(ctx, agent.PaneID); err != nil {
		restore()
		return err
	}
	if err := r.UpsertAgent(ctx, agent); err != nil {
		restore()
		return err
	}
	if defaultStatus == nil {
		return nil
	}

	statusToStore := *defaultStatus
	if existingStatus, ok := statusesSnapshot[agent.Name]; ok {
		statusToStore = existingStatus
		statusToStore.AgentName = agent.Name
	}
	if err := r.UpsertAgentStatus(ctx, statusToStore); err != nil {
		restore()
		return err
	}
	return nil
}

func (r *testAgentRepo) UpsertAgentStatus(_ context.Context, status domain.AgentStatus) error {
	if r.statusUpsertErr != nil {
		return r.statusUpsertErr
	}
	r.statuses[status.AgentName] = status
	return nil
}

func (r *testAgentRepo) GetAgentStatus(_ context.Context, agentName string) (domain.AgentStatus, error) {
	if r.statusGetErr != nil {
		return domain.AgentStatus{}, r.statusGetErr
	}
	status, ok := r.statuses[agentName]
	if !ok {
		return domain.AgentStatus{}, domain.ErrNotFound
	}
	return status, nil
}

func (r *testAgentRepo) ListAgentStatuses(_ context.Context) ([]domain.AgentStatus, error) {
	if r.statusListErr != nil {
		return nil, r.statusListErr
	}
	statuses := make([]domain.AgentStatus, 0, len(r.statuses))
	for _, status := range r.statuses {
		statuses = append(statuses, status)
	}
	return statuses, nil
}

type testTaskRepo struct {
	tasks            map[string]domain.Task
	taskGroups       map[string]domain.TaskGroup
	taskDependencies map[string][]string
	createErr        error
	getTaskErr       error
	getTaskGroupErr  error
	listErr          error
	completeTaskErr  error
	markFailedErr    error
	ackErr           error
}

func newTestTaskRepo() *testTaskRepo {
	return &testTaskRepo{
		tasks:            make(map[string]domain.Task),
		taskGroups:       make(map[string]domain.TaskGroup),
		taskDependencies: make(map[string][]string),
	}
}

func (r *testTaskRepo) CreateTask(_ context.Context, task domain.Task) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.tasks[task.ID] = task
	return nil
}

func (r *testTaskRepo) CreateTaskGroup(_ context.Context, group domain.TaskGroup) error {
	r.taskGroups[group.ID] = group
	return nil
}

func (r *testTaskRepo) GetTaskGroup(_ context.Context, groupID string) (domain.TaskGroup, error) {
	if r.getTaskGroupErr != nil {
		return domain.TaskGroup{}, r.getTaskGroupErr
	}
	group, ok := r.taskGroups[groupID]
	if !ok {
		return domain.TaskGroup{}, domain.ErrNotFound
	}
	return group, nil
}

func (r *testTaskRepo) DeleteTaskGroup(_ context.Context, groupID string) error {
	delete(r.taskGroups, groupID)
	return nil
}

func (r *testTaskRepo) CreateTaskWithDependencies(ctx context.Context, task domain.Task, dependencyTaskIDs []string) error {
	if err := r.CreateTask(ctx, task); err != nil {
		return err
	}
	if len(dependencyTaskIDs) > 0 {
		r.taskDependencies[task.ID] = append([]string(nil), dependencyTaskIDs...)
	}
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

func (r *testTaskRepo) GetTaskDependencies(_ context.Context, taskID string) ([]string, error) {
	if dependencyTaskIDs, ok := r.taskDependencies[taskID]; ok {
		return append([]string(nil), dependencyTaskIDs...), nil
	}
	return nil, nil
}

func (r *testTaskRepo) ListTasks(_ context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	tasks := make([]domain.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !filter.Status.MatchesTaskStatus(task.Status) {
			continue
		}
		if filter.AgentName != "" && task.AgentName != filter.AgentName {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (r *testTaskRepo) ActivateReadyTasks(_ context.Context, now string, agentName string) ([]domain.Task, int, error) {
	activated := make([]domain.Task, 0)
	stillBlocked := 0
	for taskID, task := range r.tasks {
		if task.Status != domain.TaskStatusBlocked {
			continue
		}
		if agentName != "" && task.AgentName != agentName {
			continue
		}
		if task.ExpiresAt != "" && task.ExpiresAt <= now {
			task.Status = domain.TaskStatusExpired
			task.CompletedAt = now
			r.tasks[taskID] = task
			continue
		}
		ready := true
		countAsBlocked := false
		for _, dependencyTaskID := range r.taskDependencies[taskID] {
			dependencyTask, ok := r.tasks[dependencyTaskID]
			if !ok {
				task.Status = domain.TaskStatusCancelled
				task.CancelledAt = now
				task.CancelReason = "dependency task is not available"
				task.CompletedAt = now
				r.tasks[taskID] = task
				ready = false
				countAsBlocked = false
				break
			}
			switch dependencyTask.Status {
			case domain.TaskStatusCompleted:
				continue
			case domain.TaskStatusCancelled, domain.TaskStatusFailed, domain.TaskStatusAbandoned, domain.TaskStatusExpired:
				task.Status = domain.TaskStatusCancelled
				task.CancelledAt = now
				task.CancelReason = "dependency task " + dependencyTaskID + " ended with status " + string(dependencyTask.Status)
				task.CompletedAt = now
				r.tasks[taskID] = task
				ready = false
				countAsBlocked = false
				break
			default:
				ready = false
				countAsBlocked = true
			}
			if !ready {
				break
			}
		}
		if ready {
			task.Status = domain.TaskStatusPending
			r.tasks[taskID] = task
			activated = append(activated, task)
			continue
		}
		if countAsBlocked {
			stillBlocked++
		}
	}
	return activated, stillBlocked, nil
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

func (r *testTaskRepo) AcknowledgeTask(_ context.Context, taskID string, acknowledgedAt string) error {
	if r.ackErr != nil {
		return r.ackErr
	}
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	task.AcknowledgedAt = acknowledgedAt
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) CancelTask(_ context.Context, taskID string, cancelledAt string, reason string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	if task.Status == domain.TaskStatusCancelled {
		return nil
	}
	if task.Status != domain.TaskStatusPending && task.Status != domain.TaskStatusBlocked {
		return errors.New("task is not cancellable")
	}
	task.Status = domain.TaskStatusCancelled
	task.CancelledAt = cancelledAt
	task.CancelReason = reason
	task.CompletedAt = cancelledAt
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) UpdateTaskProgress(_ context.Context, taskID string, progressPct *int, progressNote *string, progressUpdatedAt string) error {
	task, ok := r.tasks[taskID]
	if !ok {
		return domain.ErrNotFound
	}
	if progressPct != nil {
		task.ProgressPct = progressPct
	}
	if progressNote != nil {
		task.ProgressNote = *progressNote
	}
	task.ProgressUpdatedAt = progressUpdatedAt
	r.tasks[taskID] = task
	return nil
}

func (r *testTaskRepo) ExpirePendingTasks(_ context.Context, now string) (int64, error) {
	var expired int64
	for id, task := range r.tasks {
		if (task.Status != domain.TaskStatusPending && task.Status != domain.TaskStatusBlocked) || task.ExpiresAt == "" {
			continue
		}
		if task.ExpiresAt <= now {
			task.Status = domain.TaskStatusExpired
			task.CompletedAt = now
			r.tasks[id] = task
			expired++
		}
	}
	return expired, nil
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
	deleteErr error
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

func (r *testMessageRepo) DeleteMessage(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.messages[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.messages, id)
	return nil
}

func (r *testMessageRepo) GetMessage(_ context.Context, id string) (domain.TaskMessage, error) {
	msg, ok := r.messages[id]
	if !ok {
		return domain.TaskMessage{}, domain.ErrNotFound
	}
	return msg, nil
}

func (r *testMessageRepo) GetResponse(_ context.Context, id string) (domain.TaskMessage, error) {
	msg, ok := r.responses[id]
	if !ok {
		return domain.TaskMessage{}, domain.ErrNotFound
	}
	return msg, nil
}

type testPaneOps struct {
	selfPane   string
	sendErr    error
	captureErr error
	listErr    error
	listPanes  []domain.PaneInfo
	sent       []sentCall
	sendFn     func(ctx context.Context, paneID string, text string) error
}

type sentCall struct {
	paneID string
	text   string
}

func (p *testPaneOps) SendKeys(ctx context.Context, paneID string, text string) error {
	if p.sendFn != nil {
		return p.sendFn(ctx, paneID, text)
	}
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
	if p.listErr != nil {
		return nil, p.listErr
	}
	if p.listPanes == nil {
		return nil, nil
	}
	return append([]domain.PaneInfo(nil), p.listPanes...), nil
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
		{"agent", NewAgentService(agents, agents, panes, panes, panes, nil).logger},
		{"dispatch", NewTaskDispatchService(agents, tasks, messages, panes, panes, nil).logger},
		{"query", NewTaskQueryService(agents, tasks, messages, panes, panes, nil).logger},
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
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	_, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "codex", PaneID: "%1"})
	if err == nil || err.Error() != "failed to register agent" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentServiceRegisterAllowsReservedNameUpdate(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

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
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

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

func TestAgentServiceRegisterInitializesIdleStatus(t *testing.T) {
	repo := newTestAgentRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	if _, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "worker", PaneID: "%1"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	status, ok := repo.statuses["worker"]
	if !ok {
		t.Fatal("expected idle status to be persisted")
	}
	if status.Status != domain.AgentWorkStatusIdle {
		t.Fatalf("status = %q, want idle", status.Status)
	}
	if status.UpdatedAt == "" {
		t.Fatal("updated_at should be populated")
	}
}

func TestAgentServiceRegisterPreservesExistingStatus(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	repo.statuses["worker"] = domain.AgentStatus{
		AgentName:     "worker",
		Status:        domain.AgentWorkStatusWorking,
		CurrentTaskID: "t-123",
		Note:          "running",
		UpdatedAt:     "2026-04-14T10:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	if _, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "worker", PaneID: "%1", Role: "reviewer"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	status := repo.statuses["worker"]
	if status.Status != domain.AgentWorkStatusWorking || status.CurrentTaskID != "t-123" || status.Note != "running" {
		t.Fatalf("status = %+v", status)
	}
}

func TestAgentServiceRegisterRejectsEmptyPaneID(t *testing.T) {
	repo := newTestAgentRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	_, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "worker", PaneID: ""})
	if err == nil || err.Error() != "pane_id is required" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.agents) != 0 || len(repo.statuses) != 0 {
		t.Fatalf("register should not mutate repository on validation failure: agents=%+v statuses=%+v", repo.agents, repo.statuses)
	}
}

func TestAgentServiceRegisterRollsBackWhenStatusPersistFails(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1", Role: "existing"}
	repo.statuses["worker"] = domain.AgentStatus{
		AgentName:     "worker",
		Status:        domain.AgentWorkStatusWorking,
		CurrentTaskID: "t-123",
		Note:          "running",
		UpdatedAt:     "2026-04-14T10:00:00Z",
	}
	repo.statusUpsertErr = errors.New("status write failed")
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	_, err := svc.Register(context.Background(), RegisterAgentCmd{Name: "worker", PaneID: "%2", Role: "reviewer"})
	if err == nil || err.Error() != "failed to register agent" {
		t.Fatalf("unexpected error: %v", err)
	}
	gotAgent := repo.agents["worker"]
	if gotAgent.PaneID != "%1" || gotAgent.Role != "existing" {
		t.Fatalf("agent should be rolled back, got %+v", gotAgent)
	}
	gotStatus := repo.statuses["worker"]
	if gotStatus.Status != domain.AgentWorkStatusWorking || gotStatus.CurrentTaskID != "t-123" || gotStatus.Note != "running" {
		t.Fatalf("status should be rolled back, got %+v", gotStatus)
	}
}

func TestAgentServiceListRemovesStalePaneRegistrations(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	repo.agents["stale"] = domain.Agent{Name: "stale", PaneID: "%9"}
	repo.agents["virtual"] = domain.Agent{Name: "virtual", PaneID: domain.VirtualPaneIDPrefix + "scheduler"}
	repo.statuses["stale"] = domain.AgentStatus{AgentName: "stale", Status: domain.AgentWorkStatusBusy}
	panes := &testPaneOps{
		selfPane: "%1",
		listPanes: []domain.PaneInfo{
			{ID: "%1", Title: "worker"},
		},
	}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 visible agents after stale cleanup, got %+v", result.Agents)
	}
	if _, ok := repo.agents["stale"]; ok {
		t.Fatalf("stale agent should be removed from repo: %+v", repo.agents)
	}
	if _, ok := repo.statuses["stale"]; ok {
		t.Fatalf("stale status should be removed from repo: %+v", repo.statuses)
	}
	if !strings.Contains(result.Warning, "removed stale agent registrations for missing panes") {
		t.Fatalf("warning = %q", result.Warning)
	}
}

func TestAgentServiceListSkipsStaleCleanupWhenPaneInspectionIsIncomplete(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	repo.agents["stale"] = domain.Agent{Name: "stale", PaneID: "%9"}
	repo.statuses["stale"] = domain.AgentStatus{AgentName: "stale", Status: domain.AgentWorkStatusBusy}
	panes := &testPaneOps{
		selfPane:  "%1",
		listPanes: []domain.PaneInfo{},
	}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, ok := repo.agents["stale"]; !ok {
		t.Fatalf("stale agent should be preserved when pane inspection is incomplete: %+v", repo.agents)
	}
	if _, ok := repo.statuses["stale"]; !ok {
		t.Fatalf("stale status should be preserved when pane inspection is incomplete: %+v", repo.statuses)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("expected both agents to remain visible, got %+v", result.Agents)
	}
	if !strings.Contains(result.Warning, "skipped stale agent cleanup because tmux pane inspection was incomplete") {
		t.Fatalf("warning = %q", result.Warning)
	}
}

func TestAgentServiceListSkipsEmptyPaneIDCleanup(t *testing.T) {
	repo := newTestAgentRepo()
	repo.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	repo.agents["broken"] = domain.Agent{Name: "broken", PaneID: ""}
	panes := &testPaneOps{
		selfPane: "%1",
		listPanes: []domain.PaneInfo{
			{ID: "%1", Title: "worker"},
		},
	}
	svc := NewAgentService(repo, repo, panes, panes, panes, discardLogger())

	result, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, ok := repo.agents["broken"]; !ok {
		t.Fatal("empty pane_id registration should not be deleted by list cleanup")
	}
	var brokenStatus domain.AgentWorkStatus
	for _, agent := range result.Agents {
		if agent.Name == "broken" {
			brokenStatus = agent.Status
		}
	}
	if brokenStatus != domain.AgentWorkStatusUnknown {
		t.Fatalf("broken agent status = %q, want unknown", brokenStatus)
	}
	if !strings.Contains(result.Warning, "found agent registrations with empty pane_id") {
		t.Fatalf("warning = %q", result.Warning)
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
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

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
	if len(messages.messages) != 0 {
		t.Fatalf("saved messages should be rolled back when task persistence fails: %+v", messages.messages)
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
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())
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
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

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
	if saved := messages.messages[task.SendMessageID]; saved.Content != "implement it" {
		t.Fatalf("saved message content = %q, want %q", saved.Content, "implement it")
	}
}

func TestTaskDispatchServiceSendReturnsRollbackErrorWhenMessageCleanupFails(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.createErr = errors.New("db busy")
	messages := newTestMessageRepo()
	messages.deleteErr = errors.New("disk error")
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName: "codex",
		FromAgent: "worker",
		Message:   "implement it",
	})
	if err == nil || err.Error() != "failed to persist task" {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages.messages) != 1 {
		t.Fatalf("message should remain when rollback fails, got %d entries", len(messages.messages))
	}
}

func TestTaskDispatchServiceSendRejectsUnknownSender(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%9"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

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

func TestTaskDispatchServiceSendCreatesBlockedTaskWhenDependsOnProvided(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName:                   "codex",
		FromAgent:                   "worker",
		Message:                     "wait for dependency",
		IncludeResponseInstructions: true,
		DependsOn:                   []string{"t-dep-1", "t-dep-2"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	task := tasks.tasks[result.TaskID]
	if task.Status != domain.TaskStatusBlocked {
		t.Fatalf("task status = %q, want blocked", task.Status)
	}
	if got := tasks.taskDependencies[result.TaskID]; !reflect.DeepEqual(got, []string{"t-dep-1", "t-dep-2"}) {
		t.Fatalf("task dependencies = %v", got)
	}
	if len(panes.sent) != 0 {
		t.Fatalf("blocked task should not be delivered yet: %+v", panes.sent)
	}
}

func TestTaskDispatchServiceSendBatchPersistsGroupAndCollectsPartialFailures(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.SendBatch(context.Background(), SendTasksCmd{
		FromAgent:  "worker",
		GroupLabel: "parallel phase3",
		Tasks: []SendTaskBatchItemCmd{
			{AgentName: "codex", Message: "task 1", IncludeResponseInstructions: true},
			{AgentName: "missing", Message: "task 2", IncludeResponseInstructions: true},
		},
	})
	if err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	if result.GroupID == "" {
		t.Fatal("GroupID should be set")
	}
	if result.AllFailed {
		t.Fatal("AllFailed should be false for partial failure")
	}
	if result.Summary.Sent != 1 || result.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.Results) != 2 {
		t.Fatalf("len(results) = %d", len(result.Results))
	}
	if _, ok := tasks.taskGroups[result.GroupID]; !ok {
		t.Fatalf("task group %q should be persisted", result.GroupID)
	}
	var groupedTask domain.Task
	for _, task := range tasks.tasks {
		groupedTask = task
	}
	if groupedTask.GroupID != result.GroupID {
		t.Fatalf("task group_id = %q, want %q", groupedTask.GroupID, result.GroupID)
	}
}

func TestTaskDispatchServiceSendBatchDeletesEmptyGroupWhenAllTargetsFail(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.SendBatch(context.Background(), SendTasksCmd{
		FromAgent:  "worker",
		GroupLabel: "all fail",
		Tasks: []SendTaskBatchItemCmd{
			{AgentName: "missing-1", Message: "task 1", IncludeResponseInstructions: true},
			{AgentName: "missing-2", Message: "task 2", IncludeResponseInstructions: true},
		},
	})
	if err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	if result.Summary.Sent != 0 || result.Summary.Failed != 2 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if !result.AllFailed {
		t.Fatal("AllFailed should be true when every target fails")
	}
	if result.GroupID != "" {
		t.Fatalf("GroupID = %q, want empty", result.GroupID)
	}
	if len(tasks.taskGroups) != 0 {
		t.Fatalf("task groups should be cleaned up, got %+v", tasks.taskGroups)
	}
}

func TestTaskDispatchServiceSendBatchKeepsGroupAndTaskIDsWhenPersistedTasksFailDelivery(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2", sendErr: errors.New("tmux failed")}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.SendBatch(context.Background(), SendTasksCmd{
		FromAgent:  "worker",
		GroupLabel: "delivery fail",
		Tasks: []SendTaskBatchItemCmd{
			{AgentName: "codex", Message: "task 1", IncludeResponseInstructions: true},
		},
	})
	if err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	if result.GroupID == "" {
		t.Fatal("GroupID should be preserved when a task was created")
	}
	if result.AllFailed {
		t.Fatal("AllFailed should be false when a task was persisted")
	}
	if result.Summary.Sent != 0 || result.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if len(result.Results) != 1 {
		t.Fatalf("len(results) = %d", len(result.Results))
	}
	if result.Results[0].TaskID == "" {
		t.Fatal("failed result should still include TaskID")
	}
	if _, ok := tasks.taskGroups[result.GroupID]; !ok {
		t.Fatalf("task group %q should remain persisted", result.GroupID)
	}
	task := tasks.tasks[result.Results[0].TaskID]
	if task.Status != domain.TaskStatusFailed {
		t.Fatalf("task status = %q, want failed", task.Status)
	}
	if task.GroupID != result.GroupID {
		t.Fatalf("task group_id = %q, want %q", task.GroupID, result.GroupID)
	}
}

func TestTaskDispatchServiceSendAllowsUnregisteredCallerPane(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%99"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendTaskCmd{
		AgentName:                   "codex",
		FromAgent:                   "worker",
		Message:                     "direct handoff",
		IncludeResponseInstructions: true,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if result.TaskID == "" {
		t.Fatal("TaskID should be set")
	}
}

func TestTaskDispatchServiceSendBatchRejectsUnregisteredCallerPane(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%99"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.SendBatch(context.Background(), SendTasksCmd{
		FromAgent: "worker",
		Tasks: []SendTaskBatchItemCmd{
			{AgentName: "codex", Message: "task 1", IncludeResponseInstructions: true},
		},
	})
	if err == nil || err.Error() != "caller is not registered" {
		t.Fatalf("SendBatch error = %v", err)
	}
}

func TestTaskDispatchServiceSendBatchExpiresPendingTasksBeforeDispatch(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%2"}
	agents.agents["codex"] = domain.Agent{Name: "codex", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-expired"] = domain.Task{
		ID:        "t-expired",
		AgentName: "worker",
		Status:    domain.TaskStatusPending,
		SentAt:    "2026-04-02T09:00:00Z",
		ExpiresAt: "2000-01-01T00:00:00Z",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%2"}
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.SendBatch(context.Background(), SendTasksCmd{
		FromAgent: "worker",
		Tasks: []SendTaskBatchItemCmd{
			{AgentName: "codex", Message: "task 1", IncludeResponseInstructions: true},
		},
	})
	if err != nil {
		t.Fatalf("SendBatch: %v", err)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("expired task status = %q, want expired", got)
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

func TestResponseServiceSendExpiresPendingTaskBeforeWrite(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-expired"] = domain.Task{
		ID:             "t-expired",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         domain.TaskStatusPending,
		ExpiresAt:      "2000-01-01T00:00:00Z",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-expired", Message: "done"})
	if err == nil || err.Error() != "task is not pending" {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("task status = %q, want expired", got)
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

func TestResponseServiceSendUsesSenderInstanceIDAfterSenderRename(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["sender-renamed"] = domain.Agent{Name: "sender-renamed", PaneID: "%9", MCPInstanceID: "mcp-sender"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:               "t-001",
		AgentName:        "worker",
		AssigneePaneID:   "%1",
		SenderName:       "sender-old",
		SenderInstanceID: "mcp-sender",
		Status:           "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "sender-renamed" || result.SentTo != "%9" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(panes.sent) != 1 || panes.sent[0].paneID != "%9" {
		t.Fatalf("unexpected sent calls: %+v", panes.sent)
	}
}

func TestResponseServiceSendFallsBackToSenderNameWhenSenderInstanceIDIsStale(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:               "t-001",
		AgentName:        "worker",
		AssigneePaneID:   "%1",
		SenderName:       "orchestrator",
		SenderInstanceID: "mcp-stale",
		Status:           "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" || result.SentTo != "%0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResponseServiceSendFallsBackToSenderNameWhenSenderInstanceIDIsAmbiguous(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0", MCPInstanceID: "mcp-shared"}
	agents.agents["other"] = domain.Agent{Name: "other", PaneID: "%8", MCPInstanceID: "mcp-shared"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:               "t-001",
		AgentName:        "worker",
		AssigneePaneID:   "%1",
		SenderName:       "orchestrator",
		SenderInstanceID: "mcp-shared",
		Status:           "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" || result.SentTo != "%0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestResponseServiceSendReturnsNotAvailableWhenSenderInstanceIDAndNameBothMiss(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:               "t-001",
		AgentName:        "worker",
		AssigneePaneID:   "%1",
		SenderName:       "missing",
		SenderInstanceID: "mcp-missing",
		Status:           "pending",
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	_, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err == nil || err.Error() != "response target is not available" {
		t.Fatalf("unexpected error: %v", err)
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

func TestResponseServiceSendAllowsTrustedCallerRecoveredByInstanceID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1", MCPInstanceID: "mcp-1"}
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

	result, err := svc.Send(WithMCPInstanceID(context.Background(), "mcp-1"), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SentToName != "orchestrator" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if tasks.tasks["t-001"].Status != domain.TaskStatusCompleted {
		t.Fatalf("task status = %s, want completed", tasks.tasks["t-001"].Status)
	}
}

func TestResponseServiceSendAllowsTrustedCallerWithoutRecovery(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["other"] = domain.Agent{Name: "other", PaneID: "%2", MCPInstanceID: "mcp-1"}
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

	result, err := svc.Send(WithMCPInstanceID(context.Background(), "mcp-1"), SendResponseCmd{TaskID: "t-001", Message: "done"})
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

func TestResponseServiceSendReturnsLastKnownStatusWhenCompletionUpdateFails(t *testing.T) {
	t.Parallel()

	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.completeTaskErr = errors.New("db down")
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		Status:         domain.TaskStatusPending,
	}
	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.Send(context.Background(), SendResponseCmd{TaskID: "t-001", Message: "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Warning == "" {
		t.Fatal("expected warning when completion update fails")
	}
	if result.TaskStatus != domain.TaskStatusPending {
		t.Fatalf("TaskStatus = %q, want pending", result.TaskStatus)
	}
	if result.CompletedAt != "" {
		t.Fatalf("CompletedAt = %q, want empty", result.CompletedAt)
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
	svc := NewTaskDispatchService(agents, tasks, messages, panes, panes, discardLogger())

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
	tasks.tasks["t-2"] = domain.Task{ID: "t-2", AgentName: "worker", Status: "blocked"}
	tasks.tasks["t-3"] = domain.Task{ID: "t-3", AgentName: "worker", Status: "completed"}
	tasks.tasks["t-4"] = domain.Task{ID: "t-4", AgentName: "worker", Status: "failed"}
	tasks.tasks["t-5"] = domain.Task{ID: "t-5", AgentName: "worker", Status: "abandoned"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ListAllTasks(context.Background(), ListAllTasksCmd{StatusFilter: "all"})
	if err != nil {
		t.Fatalf("ListAllTasks: %v", err)
	}

	if !reflect.DeepEqual([]int{result.Pending, result.Blocked, result.Completed, result.Failed, result.Abandoned, result.Cancelled, result.Expired}, []int{1, 1, 1, 1, 1, 0, 0}) {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestTaskQueryServiceListAllTasksExpiresBlockedTasks(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-expired"] = domain.Task{
		ID:        "t-expired",
		AgentName: "worker",
		Status:    domain.TaskStatusBlocked,
		ExpiresAt: "2000-01-01T00:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ListAllTasks(context.Background(), ListAllTasksCmd{})
	if err != nil {
		t.Fatalf("ListAllTasks: %v", err)
	}
	if result.Blocked != 0 {
		t.Fatalf("Blocked = %d, want 0", result.Blocked)
	}
	if result.Expired != 1 {
		t.Fatalf("Expired = %d, want 1", result.Expired)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("t-expired status = %q, want expired", got)
	}
}

func TestTaskQueryServiceGetTaskDetailIncludesResponseAndOptionalFields(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["sender"] = domain.Agent{Name: "sender", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	progressPct := 80
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:                "t-1",
		AgentName:         "worker",
		AssigneePaneID:    "%1",
		SenderPaneID:      "%0",
		SenderName:        "sender",
		SendResponseID:    "r-1",
		Status:            domain.TaskStatusCompleted,
		CompletedAt:       "2026-04-02T10:05:00Z",
		AcknowledgedAt:    "2026-04-02T10:01:00Z",
		ProgressPct:       &progressPct,
		ProgressNote:      "tests almost done",
		ProgressUpdatedAt: "2026-04-02T10:04:00Z",
		ExpiresAt:         "2026-04-02T11:00:00Z",
		GroupID:           "g-1",
	}
	tasks.taskGroups["g-1"] = domain.TaskGroup{ID: "g-1", Label: "phase3", CreatedAt: "2026-04-02T10:00:00Z"}
	tasks.taskDependencies["t-1"] = []string{"t-dep-1", "t-dep-2"}
	messages := newTestMessageRepo()
	messages.responses["r-1"] = domain.TaskMessage{ID: "r-1", Content: "all green", CreatedAt: "2026-04-02T10:05:00Z"}
	panes := &testPaneOps{selfPane: "%0"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.GetTaskDetail(context.Background(), GetTaskDetailCmd{TaskID: "t-1"})
	if err != nil {
		t.Fatalf("GetTaskDetail: %v", err)
	}
	if result.Response == nil || result.Response.Content != "all green" {
		t.Fatalf("unexpected response: %+v", result.Response)
	}
	if result.ProgressPct == nil || *result.ProgressPct != 80 {
		t.Fatalf("unexpected progress pct: %v", result.ProgressPct)
	}
	if result.AcknowledgedAt == "" || result.ExpiresAt == "" {
		t.Fatalf("expected optional fields to be populated: %+v", result)
	}
	if !reflect.DeepEqual(result.DependsOn, []string{"t-dep-1", "t-dep-2"}) {
		t.Fatalf("DependsOn = %v", result.DependsOn)
	}
	if result.GroupID != "g-1" || result.GroupLabel != "phase3" {
		t.Fatalf("unexpected group metadata: %+v", result)
	}
}

func TestTaskQueryServiceGetTaskDetailKeepsGroupIDWhenGroupRowIsMissing(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["sender"] = domain.Agent{Name: "sender", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderPaneID:   "%0",
		SenderName:     "sender",
		Status:         domain.TaskStatusPending,
		GroupID:        "g-missing",
	}
	panes := &testPaneOps{selfPane: "%0"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.GetTaskDetail(context.Background(), GetTaskDetailCmd{TaskID: "t-1"})
	if err != nil {
		t.Fatalf("GetTaskDetail: %v", err)
	}
	if result.GroupID != "g-missing" {
		t.Fatalf("GroupID = %q, want g-missing", result.GroupID)
	}
	if result.GroupLabel != "" {
		t.Fatalf("GroupLabel = %q, want empty", result.GroupLabel)
	}
}

func TestTaskQueryServiceGetTaskDetailAuthorizationPaths(t *testing.T) {
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderPaneID:   "%0",
		SenderName:     "sender",
		Status:         domain.TaskStatusPending,
	}

	tests := []struct {
		name      string
		selfPane  string
		agents    map[string]domain.Agent
		wantError string
	}{
		{
			name:     "sender can inspect detail",
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
		},
		{
			name:     "assignee can inspect detail",
			selfPane: "%1",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
		},
		{
			name:     "unrelated caller is denied",
			selfPane: "%9",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
				"other":  {Name: "other", PaneID: "%9"},
			},
			wantError: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			maps.Copy(agents.agents, tt.agents)
			panes := &testPaneOps{selfPane: tt.selfPane}
			svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

			result, err := svc.GetTaskDetail(context.Background(), GetTaskDetailCmd{TaskID: "t-1"})
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("GetTaskDetail error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetTaskDetail: %v", err)
			}
			if result.TaskID != "t-1" {
				t.Fatalf("TaskID = %q", result.TaskID)
			}
		})
	}
}

func TestTaskQueryServiceCheckReadyTasksActivatesOnlySatisfiedDependencies(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-dep-done"] = domain.Task{ID: "t-dep-done", AgentName: "worker", Status: domain.TaskStatusCompleted}
	tasks.tasks["t-dep-open"] = domain.Task{ID: "t-dep-open", AgentName: "worker", Status: domain.TaskStatusPending}
	panes := &testPaneOps{selfPane: "%1"}
	messages := newTestMessageRepo()
	messages.messages["m-ready"] = domain.TaskMessage{ID: "m-ready", Content: "run after dependency", CreatedAt: "2026-04-02T10:00:00Z"}
	tasks.tasks["t-ready"] = domain.Task{ID: "t-ready", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-ready", Status: domain.TaskStatusBlocked}
	tasks.tasks["t-blocked"] = domain.Task{ID: "t-blocked", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-blocked", Status: domain.TaskStatusBlocked}
	tasks.taskDependencies["t-ready"] = []string{"t-dep-done"}
	tasks.taskDependencies["t-blocked"] = []string{"t-dep-open"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.ActivateReadyTasks(context.Background(), ActivateReadyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if !reflect.DeepEqual(result.Activated, []ReadyTaskEntry{{TaskID: "t-ready", AgentName: "worker"}}) {
		t.Fatalf("Activated = %+v", result.Activated)
	}
	if result.StillBlocked != 1 {
		t.Fatalf("StillBlocked = %d, want 1", result.StillBlocked)
	}
	if tasks.tasks["t-ready"].Status != domain.TaskStatusPending {
		t.Fatalf("t-ready status = %q", tasks.tasks["t-ready"].Status)
	}
	if tasks.tasks["t-blocked"].Status != domain.TaskStatusBlocked {
		t.Fatalf("t-blocked status = %q", tasks.tasks["t-blocked"].Status)
	}
	if !reflect.DeepEqual(panes.sent, []sentCall{{paneID: "%1", text: "run after dependency"}}) {
		t.Fatalf("sent calls = %+v", panes.sent)
	}
}

func TestTaskQueryServiceCheckReadyTasksMarksTaskFailedWhenDeliveryFails(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-dep-done"] = domain.Task{ID: "t-dep-done", AgentName: "worker", Status: domain.TaskStatusCompleted}
	tasks.tasks["t-ready"] = domain.Task{ID: "t-ready", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-ready", Status: domain.TaskStatusBlocked}
	tasks.taskDependencies["t-ready"] = []string{"t-dep-done"}
	messages := newTestMessageRepo()
	messages.messages["m-ready"] = domain.TaskMessage{ID: "m-ready", Content: "run after dependency", CreatedAt: "2026-04-02T10:00:00Z"}
	panes := &testPaneOps{selfPane: "%1", sendErr: errors.New("tmux failed")}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.ActivateReadyTasks(context.Background(), ActivateReadyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(result.Activated) != 0 {
		t.Fatalf("Activated = %+v", result.Activated)
	}
	if result.DeliveryFailed != 1 {
		t.Fatalf("DeliveryFailed = %d, want 1", result.DeliveryFailed)
	}
	if got := tasks.tasks["t-ready"].Status; got != domain.TaskStatusFailed {
		t.Fatalf("t-ready status = %q, want failed", got)
	}
}

func TestTaskQueryServiceCheckReadyTasksMarksTaskFailedWhenMessageLookupFails(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-dep-done"] = domain.Task{ID: "t-dep-done", AgentName: "worker", Status: domain.TaskStatusCompleted}
	tasks.tasks["t-ready"] = domain.Task{ID: "t-ready", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-missing", Status: domain.TaskStatusBlocked}
	tasks.taskDependencies["t-ready"] = []string{"t-dep-done"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ActivateReadyTasks(context.Background(), ActivateReadyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(result.Activated) != 0 {
		t.Fatalf("Activated = %+v", result.Activated)
	}
	if result.DeliveryFailed != 1 {
		t.Fatalf("DeliveryFailed = %d, want 1", result.DeliveryFailed)
	}
	if got := tasks.tasks["t-ready"].Status; got != domain.TaskStatusFailed {
		t.Fatalf("t-ready status = %q, want failed", got)
	}
}

func TestTaskQueryServiceCheckReadyTasksCancelsBrokenDependencies(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-dep-cancelled"] = domain.Task{ID: "t-dep-cancelled", AgentName: "worker", Status: domain.TaskStatusCancelled}
	tasks.tasks["t-broken"] = domain.Task{ID: "t-broken", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-broken", Status: domain.TaskStatusBlocked}
	tasks.taskDependencies["t-broken"] = []string{"t-dep-cancelled"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ActivateReadyTasks(context.Background(), ActivateReadyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(result.Activated) != 0 {
		t.Fatalf("Activated = %+v", result.Activated)
	}
	if result.StillBlocked != 0 {
		t.Fatalf("StillBlocked = %d, want 0", result.StillBlocked)
	}
	if got := tasks.tasks["t-broken"].Status; got != domain.TaskStatusCancelled {
		t.Fatalf("t-broken status = %q, want cancelled", got)
	}
}

func TestTaskQueryServiceCheckReadyTasksExpiresBlockedTasksBeforeDependencyWait(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-dep-open"] = domain.Task{ID: "t-dep-open", AgentName: "worker", Status: domain.TaskStatusPending}
	tasks.tasks["t-expired"] = domain.Task{
		ID:             "t-expired",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SendMessageID:  "m-expired",
		Status:         domain.TaskStatusBlocked,
		ExpiresAt:      "2000-01-01T00:00:00Z",
	}
	tasks.taskDependencies["t-expired"] = []string{"t-dep-open"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ActivateReadyTasks(context.Background(), ActivateReadyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("ActivateReadyTasks: %v", err)
	}
	if len(result.Activated) != 0 {
		t.Fatalf("Activated = %+v", result.Activated)
	}
	if result.StillBlocked != 0 {
		t.Fatalf("StillBlocked = %d, want 0", result.StillBlocked)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("t-expired status = %q, want expired", got)
	}
}

func TestTaskUpdateServiceAcknowledgeTaskExpiresPendingTaskBeforeWrite(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-expired"] = domain.Task{
		ID:             "t-expired",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
		ExpiresAt:      "2000-01-01T00:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskUpdateService(agents, tasks, panes, discardLogger())

	_, err := svc.AcknowledgeTask(context.Background(), AcknowledgeTaskCmd{
		AgentName: "worker",
		TaskID:    "t-expired",
	})
	if err == nil || err.Error() != "task is not pending" {
		t.Fatalf("AcknowledgeTask error = %v", err)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("task status = %q, want expired", got)
	}
}

func TestTaskUpdateServiceAcknowledgeTaskIsIdempotent(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		AcknowledgedAt: "2026-04-02T10:05:00Z",
		SentAt:         "2026-04-02T10:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskUpdateService(agents, tasks, panes, discardLogger())

	result, err := svc.AcknowledgeTask(context.Background(), AcknowledgeTaskCmd{
		AgentName: "worker",
		TaskID:    "t-1",
	})
	if err != nil {
		t.Fatalf("AcknowledgeTask: %v", err)
	}
	if result.AcknowledgedAt != "2026-04-02T10:05:00Z" {
		t.Fatalf("AcknowledgedAt = %q", result.AcknowledgedAt)
	}
	if got := tasks.tasks["t-1"].AcknowledgedAt; got != "2026-04-02T10:05:00Z" {
		t.Fatalf("task acknowledgement = %q", got)
	}
}

func TestTaskUpdateServiceUpdateTaskProgressExpiresPendingTaskBeforeWrite(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-expired"] = domain.Task{
		ID:             "t-expired",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
		ExpiresAt:      "2000-01-01T00:00:00Z",
	}
	progressPct := 10
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskUpdateService(agents, tasks, panes, discardLogger())

	_, err := svc.UpdateTaskProgress(context.Background(), UpdateTaskProgressCmd{
		TaskID:      "t-expired",
		ProgressPct: &progressPct,
	})
	if err == nil || err.Error() != "task is not pending" {
		t.Fatalf("UpdateTaskProgress error = %v", err)
	}
	if got := tasks.tasks["t-expired"].Status; got != domain.TaskStatusExpired {
		t.Fatalf("task status = %q, want expired", got)
	}
}

func TestTaskUpdateServiceUpdateTaskProgressRequiresInput(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskUpdateService(agents, tasks, panes, discardLogger())

	_, err := svc.UpdateTaskProgress(context.Background(), UpdateTaskProgressCmd{TaskID: "t-1"})
	if err == nil || err.Error() != "progress_pct or progress_note is required" {
		t.Fatalf("UpdateTaskProgress error = %v", err)
	}
}

func TestTaskUpdateServiceAcknowledgeTaskAllowsTrustedCallerWithoutRecovery(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	svc := NewTaskUpdateService(agents, tasks, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.AcknowledgeTask(context.Background(), AcknowledgeTaskCmd{
		AgentName: "worker",
		TaskID:    "t-1",
	})
	if err != nil {
		t.Fatalf("AcknowledgeTask error = %v", err)
	}
	if result.AcknowledgedAt == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTaskUpdateServiceAcknowledgeTaskAllowsTrustedCallerRecoveredByInstanceID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1", MCPInstanceID: "mcp-1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	svc := NewTaskUpdateService(agents, tasks, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.AcknowledgeTask(WithMCPInstanceID(context.Background(), "mcp-1"), AcknowledgeTaskCmd{
		AgentName: "worker",
		TaskID:    "t-1",
	})
	if err != nil {
		t.Fatalf("AcknowledgeTask error = %v", err)
	}
	if result.AgentName != "worker" || result.AcknowledgedAt == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTaskUpdateServiceUpdateTaskProgressAllowsTrustedCallerWithoutRecovery(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	progressPct := 10
	svc := NewTaskUpdateService(agents, tasks, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.UpdateTaskProgress(context.Background(), UpdateTaskProgressCmd{
		TaskID:      "t-1",
		ProgressPct: &progressPct,
	})
	if err != nil {
		t.Fatalf("UpdateTaskProgress error = %v", err)
	}
	if result.ProgressPct == nil || *result.ProgressPct != 10 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTaskUpdateServiceUpdateTaskProgressAllowsTrustedCallerRecoveredByInstanceID(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1", MCPInstanceID: "mcp-1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	progressPct := 10
	svc := NewTaskUpdateService(agents, tasks, errorResolver{err: errors.New("tmux missing")}, discardLogger())

	result, err := svc.UpdateTaskProgress(WithMCPInstanceID(context.Background(), "mcp-1"), UpdateTaskProgressCmd{
		TaskID:      "t-1",
		ProgressPct: &progressPct,
	})
	if err != nil {
		t.Fatalf("UpdateTaskProgress error = %v", err)
	}
	if result.ProgressPct == nil || *result.ProgressPct != 10 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestTaskUpdateServiceUpdateTaskProgressValidatesUsecaseInputs(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		Status:         domain.TaskStatusPending,
		SentAt:         "2026-04-02T10:00:00Z",
	}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskUpdateService(agents, tasks, panes, discardLogger())

	t.Run("progress pct below range", func(t *testing.T) {
		progressPct := -1
		_, err := svc.UpdateTaskProgress(context.Background(), UpdateTaskProgressCmd{
			TaskID:      "t-1",
			ProgressPct: &progressPct,
		})
		if err == nil || err.Error() != "progress_pct must be between 0 and 100" {
			t.Fatalf("UpdateTaskProgress error = %v", err)
		}
	})

	t.Run("progress note exceeds max length", func(t *testing.T) {
		progressNote := strings.Repeat("a", MaxTaskProgressNoteLen+1)
		_, err := svc.UpdateTaskProgress(context.Background(), UpdateTaskProgressCmd{
			TaskID:       "t-1",
			ProgressNote: &progressNote,
		})
		if err == nil || err.Error() != "progress_note must be 500 characters or fewer" {
			t.Fatalf("UpdateTaskProgress error = %v", err)
		}
	})
}

func TestTaskDispatchServiceCancelTaskAuthorizationAndStatuses(t *testing.T) {
	tests := []struct {
		name       string
		task       domain.Task
		selfPane   string
		agents     map[string]domain.Agent
		wantError  string
		wantStatus domain.TaskStatus
		wantReason string
	}{
		{
			name: "sender can cancel pending task",
			task: domain.Task{
				ID:             "t-pending",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				SenderName:     "sender",
				Status:         domain.TaskStatusPending,
			},
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
			wantStatus: domain.TaskStatusCancelled,
			wantReason: "no longer needed",
		},
		{
			name: "sender can cancel blocked task",
			task: domain.Task{
				ID:             "t-blocked",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				SenderName:     "sender",
				Status:         domain.TaskStatusBlocked,
			},
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
			wantStatus: domain.TaskStatusCancelled,
			wantReason: "no longer needed",
		},
		{
			name: "unregistered sender pane can cancel",
			task: domain.Task{
				ID:             "t-unregistered-sender",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				Status:         domain.TaskStatusPending,
			},
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"worker": {Name: "worker", PaneID: "%1"},
			},
			wantStatus: domain.TaskStatusCancelled,
			wantReason: "no longer needed",
		},
		{
			name: "non sender is denied",
			task: domain.Task{
				ID:             "t-denied",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				SenderName:     "sender",
				Status:         domain.TaskStatusPending,
			},
			selfPane: "%9",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
				"other":  {Name: "other", PaneID: "%9"},
			},
			wantError: "access denied",
		},
		{
			name: "completed task is rejected",
			task: domain.Task{
				ID:             "t-completed",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				SenderName:     "sender",
				Status:         domain.TaskStatusCompleted,
			},
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
			wantError: "failed to cancel task",
		},
		{
			name: "already cancelled task is idempotent",
			task: domain.Task{
				ID:             "t-cancelled",
				AgentName:      "worker",
				AssigneePaneID: "%1",
				SenderPaneID:   "%0",
				SenderName:     "sender",
				Status:         domain.TaskStatusCancelled,
				CancelledAt:    "2026-04-02T10:01:00Z",
				CompletedAt:    "2026-04-02T10:01:00Z",
			},
			selfPane: "%0",
			agents: map[string]domain.Agent{
				"sender": {Name: "sender", PaneID: "%0"},
				"worker": {Name: "worker", PaneID: "%1"},
			},
			wantStatus: domain.TaskStatusCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			maps.Copy(agents.agents, tt.agents)
			tasks := newTestTaskRepo()
			tasks.tasks[tt.task.ID] = tt.task
			panes := &testPaneOps{selfPane: tt.selfPane}
			svc := NewTaskDispatchService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

			result, err := svc.CancelTask(context.Background(), CancelTaskCmd{
				TaskID: tt.task.ID,
				Reason: "no longer needed",
			})
			if tt.wantError != "" {
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("CancelTask error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CancelTask: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Fatalf("result status = %q, want %q", result.Status, tt.wantStatus)
			}
			if got := tasks.tasks[tt.task.ID].Status; got != tt.wantStatus {
				t.Fatalf("task status = %q, want %q", got, tt.wantStatus)
			}
			if tt.wantReason != "" && tasks.tasks[tt.task.ID].CancelReason != tt.wantReason {
				t.Fatalf("cancel reason = %q, want %q", tasks.tasks[tt.task.ID].CancelReason, tt.wantReason)
			}
		})
	}
}

func TestStatusServiceUpdateAndGetStatus(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

	updateResult, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
		AgentName:     "worker",
		Status:        domain.AgentWorkStatusBusy,
		CurrentTaskID: "t-1",
		Note:          "running tests",
	})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updateResult.Status != domain.AgentWorkStatusBusy {
		t.Fatalf("UpdateStatus result = %+v", updateResult)
	}

	getResult, err := svc.GetAgentStatus(context.Background(), GetAgentStatusCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("GetAgentStatus: %v", err)
	}
	if getResult.Status != domain.AgentWorkStatusBusy || getResult.CurrentTaskID != "t-1" || getResult.Note != "running tests" {
		t.Fatalf("GetAgentStatus result = %+v", getResult)
	}
	if getResult.SecondsSinceUpdate == nil {
		t.Fatalf("SecondsSinceUpdate should be populated")
	}
}

func TestStatusServiceErrorPaths(t *testing.T) {
	t.Run("update status rejects unregistered caller", func(t *testing.T) {
		agents := newTestAgentRepo()
		agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
		panes := &testPaneOps{selfPane: "%9"}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		_, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
			AgentName: "worker",
			Status:    domain.AgentWorkStatusBusy,
		})
		if err == nil || err.Error() != "caller is not registered" {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("update status fails when target agent does not exist", func(t *testing.T) {
		agents := newTestAgentRepo()
		panes := &testPaneOps{selfPane: ""}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		_, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
			AgentName: "missing",
			Status:    domain.AgentWorkStatusBusy,
		})
		if err == nil || err.Error() != "agent is not available" {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("update status rejects mismatched non-trusted caller", func(t *testing.T) {
		agents := newTestAgentRepo()
		agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
		agents.agents["other"] = domain.Agent{Name: "other", PaneID: "%2"}
		panes := &testPaneOps{selfPane: "%2"}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		_, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
			AgentName: "worker",
			Status:    domain.AgentWorkStatusBusy,
		})
		if err == nil || err.Error() != "access denied" {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("update status propagates repository failure", func(t *testing.T) {
		agents := newTestAgentRepo()
		agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
		agents.statusUpsertErr = errors.New("db busy")
		panes := &testPaneOps{selfPane: "%1"}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		_, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
			AgentName: "worker",
			Status:    domain.AgentWorkStatusBusy,
		})
		if err == nil || err.Error() != "failed to update agent status" {
			t.Fatalf("UpdateStatus error = %v", err)
		}
	})

	t.Run("get status returns idle when unset", func(t *testing.T) {
		agents := newTestAgentRepo()
		agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
		panes := &testPaneOps{selfPane: "%1"}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		result, err := svc.GetAgentStatus(context.Background(), GetAgentStatusCmd{AgentName: "worker"})
		if err != nil {
			t.Fatalf("GetAgentStatus: %v", err)
		}
		if result.Status != domain.AgentWorkStatusIdle {
			t.Fatalf("status = %q, want idle", result.Status)
		}
	})

	t.Run("get status propagates unexpected repository failure", func(t *testing.T) {
		agents := newTestAgentRepo()
		agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
		agents.statusGetErr = errors.New("db busy")
		panes := &testPaneOps{selfPane: "%1"}
		svc := NewStatusService(agents, agents, nil, nil, nil, panes, discardLogger())

		_, err := svc.GetAgentStatus(context.Background(), GetAgentStatusCmd{AgentName: "worker"})
		if err == nil || err.Error() != "failed to load agent status" {
			t.Fatalf("GetAgentStatus error = %v", err)
		}
	})
}

func TestTaskQueryServiceGetMyTasksIncludesTaskIDPlaceholderInstruction(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}
	tasks := newTestTaskRepo()
	tasks.tasks["t-1"] = domain.Task{ID: "t-1", AgentName: "worker", Status: "pending"}
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

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

func TestTaskQueryServiceGetTaskMessage(t *testing.T) {
	tests := []struct {
		name        string
		callerPane  string
		instanceID  string
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
			name:        "trusted caller can read by declared assignee name",
			callerPane:  "",
			agentName:   "worker",
			msgID:       "m-001",
			setupAgent:  map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1"}},
			setupTask:   map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", SendMessageID: "m-001", Status: "pending", SentAt: "2026-03-07T10:00:00Z"}},
			setupMsg:    map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "trusted", CreatedAt: "2026-03-07T10:00:00Z"}},
			wantTaskID:  "t-001",
			wantContent: "trusted",
		},
		{
			name:        "trusted caller is recovered by instance id",
			callerPane:  "",
			instanceID:  "mcp-1",
			agentName:   "worker",
			msgID:       "m-001",
			setupAgent:  map[string]domain.Agent{"worker": {Name: "worker", PaneID: "%1", MCPInstanceID: "mcp-1"}},
			setupTask:   map[string]domain.Task{"t-001": {ID: "t-001", AgentName: "worker", AssigneePaneID: "%1", SendMessageID: "m-001", Status: "pending", SentAt: "2026-03-07T10:00:00Z"}},
			setupMsg:    map[string]domain.TaskMessage{"m-001": {ID: "m-001", Content: "trusted", CreatedAt: "2026-03-07T10:00:00Z"}},
			wantTaskID:  "t-001",
			wantContent: "trusted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := &testAgentRepo{agents: tt.setupAgent}
			tasks := &testTaskRepo{tasks: tt.setupTask}
			messages := &testMessageRepo{messages: tt.setupMsg, responses: make(map[string]domain.TaskMessage)}
			panes := &testPaneOps{selfPane: tt.callerPane}
			svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

			ctx := context.Background()
			if tt.instanceID != "" {
				ctx = WithMCPInstanceID(ctx, tt.instanceID)
			}

			result, err := svc.GetTaskMessage(ctx, GetTaskMessageCmd{
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
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

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
	svc := NewTaskQueryService(agents, tasks, newTestMessageRepo(), panes, panes, discardLogger())

	result, err := svc.ListAllTasks(context.Background(), ListAllTasksCmd{StatusFilter: "all"})
	if err != nil {
		t.Fatalf("ListAllTasks: %v", err)
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

// ---------------------------------------------------------------------------
// Feature 1: GetMyTasks with inline message delivery
// ---------------------------------------------------------------------------

func TestGetMyTasksInlineMessageDelivery(t *testing.T) {
	tests := []struct {
		name                    string
		tasks                   map[string]domain.Task
		messages                map[string]domain.TaskMessage
		maxInline               int
		wantInlineCount         int
		wantAcknowledged        []string // taskIDs that should remain acknowledged
		wantNewAcknowledgements int
		wantInlineContent       []string // expected inline message contents
		wantInlineFromAgent     []string
		wantErr                 bool
	}{
		{
			name: "pending unacknowledged task returns inline message and auto-acknowledges it",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Hello, task-1", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			maxInline:               3,
			wantInlineCount:         1,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 1,
			wantInlineContent:       []string{"Hello, task-1"},
			wantInlineFromAgent:     []string{"orchestrator"},
		},
		{
			name: "already acknowledged task does not return inline message",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "2026-01-01T11:00:00Z",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Hello, task-1", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			maxInline:               3,
			wantInlineCount:         0,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 0,
		},
		{
			name: "non-pending task (completed) does not return inline message",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusCompleted,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
					CompletedAt:    "2026-01-01T12:30:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Hello, task-1", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			maxInline:               3,
			wantInlineCount:         0,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 0,
		},
		{
			name: "MaxInline=1 limits inline messages to 1 out of multiple pending tasks",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
				"t-2": {
					ID:             "t-2",
					AgentName:      "agent-a",
					SenderName:     "reviewer",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-2",
					SentAt:         "2026-01-01T12:01:00Z",
				},
				"t-3": {
					ID:             "t-3",
					AgentName:      "agent-a",
					SenderName:     "planner",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-3",
					SentAt:         "2026-01-01T12:02:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task 1", CreatedAt: "2026-01-01T12:00:00Z"},
				"m-2": {ID: "m-2", Content: "Task 2", CreatedAt: "2026-01-01T12:01:00Z"},
				"m-3": {ID: "m-3", Content: "Task 3", CreatedAt: "2026-01-01T12:02:00Z"},
			},
			maxInline:               1,
			wantInlineCount:         1,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 1,
		},
		{
			name: "message fetch failure logs but task still listed without inline",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-missing",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages:                map[string]domain.TaskMessage{},
			maxInline:               3,
			wantInlineCount:         0,
			wantAcknowledged:        []string{}, // Fetch failures must not mutate acknowledgment state
			wantNewAcknowledgements: 0,
		},
		{
			name: "task without SendMessageID does not return inline message",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "", // No message ID
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages:                map[string]domain.TaskMessage{},
			maxInline:               3,
			wantInlineCount:         0,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 0,
		},
		{
			name: "MaxInline=0 defaults to DefaultMaxInline",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					SenderName:     "orchestrator",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
				"t-2": {
					ID:             "t-2",
					AgentName:      "agent-a",
					SenderName:     "reviewer",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-2",
					SentAt:         "2026-01-01T12:01:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task 1", CreatedAt: "2026-01-01T12:00:00Z"},
				"m-2": {ID: "m-2", Content: "Task 2", CreatedAt: "2026-01-01T12:01:00Z"},
			},
			maxInline:               0,
			wantInlineCount:         2,
			wantAcknowledged:        []string{},
			wantNewAcknowledgements: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

			tasks := newTestTaskRepo()
			maps.Copy(tasks.tasks, tt.tasks)

			messages := newTestMessageRepo()
			maps.Copy(messages.messages, tt.messages)

			panes := &testPaneOps{selfPane: "%1"}
			svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())
			initialAcknowledgedCount := 0
			for _, task := range tasks.tasks {
				if task.AcknowledgedAt != "" {
					initialAcknowledgedCount++
				}
			}

			result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{
				AgentName:    "agent-a",
				StatusFilter: "all",
				MaxInline:    tt.maxInline,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetMyTasks: %v", err)
			}

			// Verify inline message count
			if len(result.InlineMessages) != tt.wantInlineCount {
				t.Errorf("inline messages count = %d, want %d", len(result.InlineMessages), tt.wantInlineCount)
			}

			// Verify inline message content
			for i, wantContent := range tt.wantInlineContent {
				if i >= len(result.InlineMessages) {
					t.Errorf("expected inline message %d not found", i)
					continue
				}
				if result.InlineMessages[i].Content != wantContent {
					t.Errorf("inline message[%d].Content = %q, want %q", i, result.InlineMessages[i].Content, wantContent)
				}
			}
			for i, wantFromAgent := range tt.wantInlineFromAgent {
				if i >= len(result.InlineMessages) {
					t.Errorf("expected inline message %d for from_agent check not found", i)
					continue
				}
				if result.InlineMessages[i].FromAgent != wantFromAgent {
					t.Errorf("inline message[%d].FromAgent = %q, want %q", i, result.InlineMessages[i].FromAgent, wantFromAgent)
				}
			}

			// Verify acknowledgment state
			for _, taskID := range tt.wantAcknowledged {
				task, ok := tasks.tasks[taskID]
				if !ok {
					t.Errorf("task %s not found after GetMyTasks", taskID)
					continue
				}
				if task.AcknowledgedAt == "" {
					t.Errorf("task %s should remain acknowledged", taskID)
				}
			}

			acknowledgedCount := 0
			for _, task := range tasks.tasks {
				if task.AcknowledgedAt != "" {
					acknowledgedCount++
				}
			}
			newAcknowledgements := acknowledgedCount - initialAcknowledgedCount
			if newAcknowledgements != tt.wantNewAcknowledgements {
				t.Errorf("new acknowledged task count = %d, want %d", newAcknowledgements, tt.wantNewAcknowledgements)
			}

			// Verify tasks are still listed
			if len(result.Tasks) != len(tt.tasks) {
				t.Errorf("tasks count = %d, want %d", len(result.Tasks), len(tt.tasks))
			}
		})
	}
}

func TestGetMyTasksAccessControl(t *testing.T) {
	tests := []struct {
		name       string
		caller     domain.Agent
		instanceID string
		agentName  string
		wantAccess bool
	}{
		{
			name:       "trusted caller can access any declared assignee inbox",
			caller:     domain.Agent{Name: "_trusted"},
			agentName:  "agent-a",
			wantAccess: true,
		},
		{
			name:       "self-agent can access own tasks",
			caller:     domain.Agent{Name: "agent-a", PaneID: "%1"},
			agentName:  "agent-a",
			wantAccess: true,
		},
		{
			name:       "trusted caller recovered by instance can access own tasks",
			caller:     domain.Agent{Name: "_trusted"},
			instanceID: "mcp-a",
			agentName:  "agent-a",
			wantAccess: true,
		},
		{
			name:       "other agent cannot access different agent's tasks",
			caller:     domain.Agent{Name: "agent-b", PaneID: "%2"},
			agentName:  "agent-a",
			wantAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1", MCPInstanceID: "mcp-a"}
			agents.agents["agent-b"] = domain.Agent{Name: "agent-b", PaneID: "%2"}

			tasks := newTestTaskRepo()
			messages := newTestMessageRepo()

			// Mock the resolver to return the specific caller
			mockResolver := &mockCallerResolver{agent: tt.caller}
			svc := NewTaskQueryService(agents, tasks, messages, mockResolver, mockResolver, discardLogger())

			ctx := context.Background()
			if tt.instanceID != "" {
				ctx = WithMCPInstanceID(ctx, tt.instanceID)
			}

			result, err := svc.GetMyTasks(ctx, GetMyTasksCmd{
				AgentName: tt.agentName,
			})

			if tt.wantAccess {
				if err != nil {
					t.Fatalf("expected access granted, got error: %v", err)
				}
				if result.AgentName != tt.agentName {
					t.Errorf("result.AgentName = %q, want %q", result.AgentName, tt.agentName)
				}
			} else {
				if err == nil || err.Error() != "access denied" {
					t.Errorf("expected 'access denied' error, got: %v", err)
				}
			}
		})
	}
}

type mockCallerResolver struct {
	agent domain.Agent
}

func (m *mockCallerResolver) GetPaneID(context.Context) (string, error) {
	return m.agent.PaneID, nil
}

func (m *mockCallerResolver) SendKeys(context.Context, string, string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Feature 2: UpdateStatus with idle re-delivery
// ---------------------------------------------------------------------------

type taskRepoWithAckError struct {
	*testTaskRepo
	ackErr error
}

func (r *taskRepoWithAckError) AcknowledgeTask(_ context.Context, taskID string, acknowledgedAt string) error {
	if r.ackErr != nil {
		return r.ackErr
	}
	return r.testTaskRepo.AcknowledgeTask(context.Background(), taskID, acknowledgedAt)
}

func TestGetMyTasksAutoAcknowledgesInlineMessages(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

	baseTaskRepo := newTestTaskRepo()
	baseTaskRepo.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "agent-a",
		Status:         domain.TaskStatusPending,
		AcknowledgedAt: "",
		SendMessageID:  "m-1",
	}

	tasks := baseTaskRepo

	messages := newTestMessageRepo()
	messages.messages["m-1"] = domain.TaskMessage{ID: "m-1", Content: "Task content"}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{
		AgentName: "agent-a",
	})

	if err != nil {
		t.Fatalf("GetMyTasks: %v", err)
	}

	if len(result.InlineMessages) != 1 {
		t.Fatalf("expected 1 inline message, got %d", len(result.InlineMessages))
	}
	if got := tasks.tasks["t-1"].AcknowledgedAt; got == "" {
		t.Fatal("AcknowledgedAt should be set")
	}
}

func TestGetMyTasksReturnsInlineMessagesWhenAutoAcknowledgeFails(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

	baseTaskRepo := newTestTaskRepo()
	baseTaskRepo.tasks["t-1"] = domain.Task{
		ID:             "t-1",
		AgentName:      "agent-a",
		Status:         domain.TaskStatusPending,
		AcknowledgedAt: "",
		SendMessageID:  "m-1",
	}

	tasks := &taskRepoWithAckError{
		testTaskRepo: baseTaskRepo,
		ackErr:       errors.New("acknowledge failed"),
	}

	messages := newTestMessageRepo()
	messages.messages["m-1"] = domain.TaskMessage{ID: "m-1", Content: "Task content"}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger())

	result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{
		AgentName: "agent-a",
	})
	if err != nil {
		t.Fatalf("GetMyTasks: %v", err)
	}
	if len(result.InlineMessages) != 1 {
		t.Fatalf("expected 1 inline message, got %d", len(result.InlineMessages))
	}
	if got := tasks.tasks["t-1"].AcknowledgedAt; got != "" {
		t.Fatalf("AcknowledgedAt = %q, want empty after failed auto-ack", got)
	}
}

func TestUpdateStatusRedeliveryWithMissingDependencies(t *testing.T) {
	tests := []struct {
		name                 string
		tasksNil             bool
		messagesNil          bool
		senderNil            bool
		getAgentErr          error
		listTasksErr         error
		wantRedeliveredCount int
	}{
		{
			name:                 "nil tasks returns 0 without error",
			tasksNil:             true,
			wantRedeliveredCount: 0,
		},
		{
			name:                 "nil messages returns 0 without error",
			messagesNil:          true,
			wantRedeliveredCount: 0,
		},
		{
			name:                 "nil sender returns 0 without error",
			senderNil:            true,
			wantRedeliveredCount: 0,
		},
		{
			name:                 "GetAgent error returns 0",
			getAgentErr:          errors.New("db error"),
			wantRedeliveredCount: 0,
		},
		{
			name:                 "ListTasks error returns 0",
			listTasksErr:         errors.New("db error"),
			wantRedeliveredCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			if tt.getAgentErr != nil {
				agents.getAgentErr = tt.getAgentErr
			}
			agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

			taskRepo := newTestTaskRepo()
			if tt.listTasksErr != nil {
				taskRepo.listErr = tt.listTasksErr
			}
			taskRepo.tasks["t-1"] = domain.Task{
				ID:             "t-1",
				AgentName:      "agent-a",
				Status:         domain.TaskStatusPending,
				AcknowledgedAt: "",
				SendMessageID:  "m-1",
			}

			messages := newTestMessageRepo()
			messages.messages["m-1"] = domain.TaskMessage{ID: "m-1", Content: "Task content"}

			panes := &testPaneOps{selfPane: "%1"}
			svc := NewStatusService(agents, agents, taskRepo, messages, panes, panes, discardLogger())

			// Override dependencies as needed
			if tt.tasksNil {
				svc.tasks = nil
			}
			if tt.messagesNil {
				svc.messages = nil
			}
			if tt.senderNil {
				svc.sender = nil
			}

			result, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
				AgentName: "agent-a",
				Status:    domain.AgentWorkStatusIdle,
			})

			if err != nil && !strings.Contains(err.Error(), "agent is not available") {
				t.Fatalf("UpdateStatus: %v", err)
			}

			if result.RedeliveredCount != tt.wantRedeliveredCount {
				t.Errorf("RedeliveredCount = %d, want %d", result.RedeliveredCount, tt.wantRedeliveredCount)
			}
		})
	}
}

func TestUpdateStatusIdleRedelivery(t *testing.T) {
	tests := []struct {
		name                    string
		status                  domain.AgentWorkStatus
		tasks                   map[string]domain.Task
		messages                map[string]domain.TaskMessage
		agentName               string
		paneID                  string
		wantRedeliveredCount    int
		wantRedeliveryFailures  int
		wantSendKeysCallCount   int
		wantNewAcknowledgements int
		wantSentTo              []string // pane IDs that should receive re-delivered messages
		sendKeysErr             error
		ackErr                  error
		wantErr                 bool
	}{
		{
			name:   "idle status with unacknowledged pending task re-delivers and returns count=1",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			wantRedeliveredCount:    1,
			wantSendKeysCallCount:   1,
			wantNewAcknowledgements: 1,
			wantSentTo:              []string{"%1"},
		},
		{
			name:   "busy status does not re-deliver and returns count=0",
			status: domain.AgentWorkStatusBusy,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			wantRedeliveredCount:    0,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
		{
			name:   "idle status with already acknowledged task does not re-deliver",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "2026-01-01T11:00:00Z",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			wantRedeliveredCount:    0,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
		{
			name:      "idle status with virtual pane agent does not re-deliver",
			status:    domain.AgentWorkStatusIdle,
			agentName: "virtual-agent",
			paneID:    domain.VirtualPaneIDPrefix + "virtual",
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "virtual-agent",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			wantRedeliveredCount:    0,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
		{
			name:   "message fetch failure in redeliver logs error and returns count=0",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-missing",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages:                map[string]domain.TaskMessage{},
			wantRedeliveredCount:    0,
			wantRedeliveryFailures:  1,
			wantSendKeysCallCount:   0, // SendKeys not called because message fetch failed first
			wantNewAcknowledgements: 0,
		},
		{
			name:   "send failure in redeliver leaves task pending for retry",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			sendKeysErr:             errors.New("send failed"),
			wantRedeliveredCount:    0,
			wantRedeliveryFailures:  1,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
		{
			name:   "acknowledge failure after redelivery leaves task pending for retry",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			ackErr:                  errors.New("ack failed"),
			wantRedeliveredCount:    0,
			wantRedeliveryFailures:  1,
			wantSendKeysCallCount:   1,
			wantNewAcknowledgements: 0,
		},
		{
			name:   "idle status with no pending tasks returns count=0",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusCompleted,
					AcknowledgedAt: "2026-01-01T11:00:00Z",
					SendMessageID:  "m-1",
					CompletedAt:    "2026-01-01T12:30:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task content", CreatedAt: "2026-01-01T12:00:00Z"},
			},
			wantRedeliveredCount:    0,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
		{
			name:   "idle status with multiple pending tasks re-delivers all unacknowledged",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-1",
					SentAt:         "2026-01-01T12:00:00Z",
				},
				"t-2": {
					ID:             "t-2",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "m-2",
					SentAt:         "2026-01-01T12:01:00Z",
				},
				"t-3": {
					ID:             "t-3",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "2026-01-01T11:00:00Z",
					SendMessageID:  "m-3",
					SentAt:         "2026-01-01T12:02:00Z",
				},
			},
			messages: map[string]domain.TaskMessage{
				"m-1": {ID: "m-1", Content: "Task 1", CreatedAt: "2026-01-01T12:00:00Z"},
				"m-2": {ID: "m-2", Content: "Task 2", CreatedAt: "2026-01-01T12:01:00Z"},
				"m-3": {ID: "m-3", Content: "Task 3", CreatedAt: "2026-01-01T12:02:00Z"},
			},
			wantRedeliveredCount:    2, // t-1 and t-2 are unacknowledged
			wantSendKeysCallCount:   2,
			wantNewAcknowledgements: 2,
		},
		{
			name:   "idle status with pending task without SendMessageID is skipped",
			status: domain.AgentWorkStatusIdle,
			tasks: map[string]domain.Task{
				"t-1": {
					ID:             "t-1",
					AgentName:      "agent-a",
					Status:         domain.TaskStatusPending,
					AcknowledgedAt: "",
					SendMessageID:  "", // No message ID
					SentAt:         "2026-01-01T12:00:00Z",
				},
			},
			messages:                map[string]domain.TaskMessage{},
			wantRedeliveredCount:    0,
			wantSendKeysCallCount:   0,
			wantNewAcknowledgements: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := newTestAgentRepo()
			agentName := tt.agentName
			if agentName == "" {
				agentName = "agent-a"
			}
			selfPane := tt.paneID
			if selfPane == "" {
				selfPane = "%1"
			}
			agents.agents[agentName] = domain.Agent{Name: agentName, PaneID: selfPane}

			taskRepo := newTestTaskRepo()
			maps.Copy(taskRepo.tasks, tt.tasks)
			taskRepo.ackErr = tt.ackErr

			messages := newTestMessageRepo()
			maps.Copy(messages.messages, tt.messages)

			panes := &testPaneOps{selfPane: selfPane, sendErr: tt.sendKeysErr}
			svc := NewStatusService(agents, agents, taskRepo, messages, panes, panes, discardLogger())
			initialAcknowledgedCount := 0
			for _, task := range taskRepo.tasks {
				if task.AcknowledgedAt != "" {
					initialAcknowledgedCount++
				}
			}

			result, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
				AgentName: agentName,
				Status:    tt.status,
			})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateStatus: %v", err)
			}

			if result.RedeliveredCount != tt.wantRedeliveredCount {
				t.Errorf("RedeliveredCount = %d, want %d", result.RedeliveredCount, tt.wantRedeliveredCount)
			}
			if result.RedeliveryFailedCount != tt.wantRedeliveryFailures {
				t.Errorf("RedeliveryFailedCount = %d, want %d", result.RedeliveryFailedCount, tt.wantRedeliveryFailures)
			}

			if len(panes.sent) != tt.wantSendKeysCallCount {
				t.Errorf("SendKeys call count = %d, want %d", len(panes.sent), tt.wantSendKeysCallCount)
			}
			acknowledgedCount := 0
			for _, task := range taskRepo.tasks {
				if task.AcknowledgedAt != "" {
					acknowledgedCount++
				}
			}
			newAcknowledgements := acknowledgedCount - initialAcknowledgedCount
			if newAcknowledgements != tt.wantNewAcknowledgements {
				t.Errorf("new acknowledged task count = %d, want %d", newAcknowledgements, tt.wantNewAcknowledgements)
			}

			for i, wantPaneID := range tt.wantSentTo {
				if i >= len(panes.sent) {
					t.Errorf("expected sent call %d to pane %q not found", i, wantPaneID)
					continue
				}
				if panes.sent[i].paneID != wantPaneID {
					t.Errorf("sent[%d].paneID = %q, want %q", i, panes.sent[i].paneID, wantPaneID)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Feature 3: Bootstrap message enhancement with idle behavior section
// ---------------------------------------------------------------------------

func TestUpdateStatusReturnsCorrectStatusField(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewStatusService(agents, agents, newTestTaskRepo(), newTestMessageRepo(), panes, panes, discardLogger())

	tests := []struct {
		name        string
		inputStatus domain.AgentWorkStatus
	}{
		{"busy status", domain.AgentWorkStatusBusy},
		{"idle status", domain.AgentWorkStatusIdle},
		{"working status", domain.AgentWorkStatusWorking},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
				AgentName: "agent-a",
				Status:    tt.inputStatus,
			})

			if err != nil {
				t.Fatalf("UpdateStatus: %v", err)
			}

			if result.Status != tt.inputStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.inputStatus)
			}
		})
	}
}

func TestUpdateStatusRejectsInvalidStatus(t *testing.T) {
	agents := newTestAgentRepo()
	agents.agents["agent-a"] = domain.Agent{Name: "agent-a", PaneID: "%1"}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewStatusService(agents, agents, newTestTaskRepo(), newTestMessageRepo(), panes, panes, discardLogger())

	_, err := svc.UpdateStatus(context.Background(), UpdateStatusCmd{
		AgentName: "agent-a",
		Status:    "offline",
	})
	if err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestBuildMemberBootstrapMessageIncludesIdleBehaviorSection(t *testing.T) {
	tests := []struct {
		name              string
		teamName          string
		paneTitle         string
		role              string
		customMessage     string
		skills            []domain.Skill
		wantIdleSection   bool
		wantPollingText   bool
		wantStatusText    bool
		wantGetMyTasksRef bool
	}{
		{
			name:              "bootstrap message contains idle behavior section",
			teamName:          "テストチーム",
			paneTitle:         "TestAgent",
			role:              "worker",
			wantIdleSection:   true,
			wantPollingText:   true,
			wantStatusText:    true,
			wantGetMyTasksRef: true,
		},
		{
			name:              "idle section includes polling instruction",
			teamName:          "チーム",
			paneTitle:         "Agent",
			role:              "member",
			wantIdleSection:   true,
			wantPollingText:   true,
			wantGetMyTasksRef: true,
		},
		{
			name:            "idle section includes status reporting instruction",
			teamName:        "チーム",
			paneTitle:       "Agent",
			role:            "member",
			wantIdleSection: true,
			wantStatusText:  true,
		},
		{
			name:              "idle section includes get_my_tasks reference",
			teamName:          "チーム",
			paneTitle:         "Agent",
			role:              "member",
			wantIdleSection:   true,
			wantGetMyTasksRef: true,
		},
		{
			name:            "bootstrap message with custom message includes idle section",
			teamName:        "チーム",
			paneTitle:       "Agent",
			role:            "member",
			customMessage:   "Custom instructions here",
			wantIdleSection: true,
			wantPollingText: true,
		},
		{
			name:      "bootstrap message with skills includes idle section",
			teamName:  "チーム",
			paneTitle: "Agent",
			role:      "member",
			skills: []domain.Skill{
				{Name: "golang", Description: "Go programming"},
				{Name: "testing", Description: "Test writing"},
			},
			wantIdleSection: true,
			wantPollingText: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := AddMemberCmd{
				PaneTitle:     tt.paneTitle,
				Role:          tt.role,
				CustomMessage: tt.customMessage,
				Skills:        tt.skills,
			}

			msg := buildMemberBootstrapMessage(tt.teamName, cmd, "%1", "test-agent")

			if tt.wantIdleSection {
				if !strings.Contains(msg, "アイドル時の行動（重要）") {
					t.Errorf("bootstrap message missing idle behavior section header")
				}
			}

			if tt.wantPollingText {
				if !strings.Contains(msg, "30-60秒ごと") {
					t.Errorf("bootstrap message missing polling interval guidance")
				}
			}

			if tt.wantStatusText {
				if !strings.Contains(msg, "update_status") {
					t.Errorf("bootstrap message missing update_status instruction")
				}
			}

			if tt.wantGetMyTasksRef {
				if !strings.Contains(msg, "get_my_tasks") {
					t.Errorf("bootstrap message missing get_my_tasks reference")
				}
			}

			// Verify idle section appears after workflow section
			if tt.wantIdleSection {
				workflowIdx := strings.Index(msg, "--- ワークフロー ---")
				idleIdx := strings.Index(msg, "--- アイドル時の行動（重要） ---")
				if workflowIdx >= 0 && idleIdx >= 0 && workflowIdx >= idleIdx {
					t.Errorf("idle behavior section should appear after workflow section")
				}
			}

			// Verify team name is included
			if !strings.Contains(msg, tt.teamName) {
				t.Errorf("bootstrap message missing team name: %q", tt.teamName)
			}

			// Verify role is included
			if !strings.Contains(msg, tt.role) {
				t.Errorf("bootstrap message missing role: %q", tt.role)
			}

			// Verify custom message is included if provided
			if tt.customMessage != "" && !strings.Contains(msg, tt.customMessage) {
				t.Errorf("bootstrap message missing custom message: %q", tt.customMessage)
			}

			// Verify skills are included if provided
			if len(tt.skills) > 0 {
				for _, skill := range tt.skills {
					if !strings.Contains(msg, skill.Name) {
						t.Errorf("bootstrap message missing skill: %q", skill.Name)
					}
				}
			}
		})
	}
}
