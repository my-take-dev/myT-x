package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestTaskQueryServiceGetMyTasksKeepsStoredPayloadTasksUnacknowledged(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	agents := newTestAgentRepo()
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}

	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		Status:         domain.TaskStatusPending,
		AcknowledgedAt: "",
		SendMessageID:  "m-001",
	}

	messages := newTestMessageRepo()
	messagePath := filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-001__m-001.md"))
	messages.messages["m-001"] = domain.TaskMessage{
		ID:             "m-001",
		CreatedAt:      "2026-04-18T01:02:03Z",
		StorageMode:    domain.MessageStorageFile,
		ContentPreview: "stored preview",
		ArtifactPaths:  []string{messagePath},
		PartCount:      1,
		ContentChars:   MaxInlineDeliveryChars + 1,
		SHA256:         "abc123",
	}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger(), projectRoot)

	result, err := svc.GetMyTasks(context.Background(), GetMyTasksCmd{AgentName: "worker"})
	if err != nil {
		t.Fatalf("GetMyTasks: %v", err)
	}
	if len(result.InlineMessages) != 1 {
		t.Fatalf("inline_messages len = %d, want 1", len(result.InlineMessages))
	}
	if got := tasks.tasks["t-001"].AcknowledgedAt; got != "" {
		t.Fatalf("AcknowledgedAt = %q, want empty for stored payload metadata", got)
	}
	wantResolvedPath := filepath.Join(projectRoot, ".myT-x", "orchestrator", "payloads", "t-001__m-001.md")
	if got := result.InlineMessages[0].ArtifactPaths[0]; got != wantResolvedPath {
		t.Fatalf("artifact_path = %q, want %q", got, wantResolvedPath)
	}
}

func TestTaskQueryServiceGetTaskDetailResolvesResponseArtifactPathsToAbsolutePaths(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	agents := newTestAgentRepo()
	agents.agents["orchestrator"] = domain.Agent{Name: "orchestrator", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}

	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "orchestrator",
		SenderPaneID:   "%0",
		Status:         domain.TaskStatusCompleted,
		SendResponseID: "r-001",
	}

	messages := newTestMessageRepo()
	manifestPath := filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-001__r-001__manifest.json"))
	partPath := filepath.ToSlash(filepath.Join(payloadRelativeDir, "t-001__r-001__p001-of002.md"))
	messages.responses["r-001"] = domain.TaskMessage{
		ID:             "r-001",
		CreatedAt:      "2026-04-18T01:02:03Z",
		StorageMode:    domain.MessageStorageMultipartFile,
		ContentPreview: "response preview",
		ArtifactPaths: []string{
			manifestPath,
			partPath,
		},
		PartCount:    1,
		ContentChars: MaxInlineDeliveryChars + 2,
		SHA256:       "def456",
	}

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewTaskQueryService(agents, tasks, messages, panes, panes, discardLogger(), projectRoot)

	result, err := svc.GetTaskDetail(context.Background(), GetTaskDetailCmd{TaskID: "t-001"})
	if err != nil {
		t.Fatalf("GetTaskDetail: %v", err)
	}
	if result.Response == nil {
		t.Fatal("Response = nil, want stored response metadata")
	}
	wantResolvedPath := filepath.Join(projectRoot, ".myT-x", "orchestrator", "payloads", "t-001__r-001__manifest.json")
	if got := result.Response.ArtifactPaths[0]; got != wantResolvedPath {
		t.Fatalf("response artifact path = %q, want %q", got, wantResolvedPath)
	}
}
