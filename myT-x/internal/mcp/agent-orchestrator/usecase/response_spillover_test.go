package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestResponseServiceSendReturnsErrorAndCleansArtifactsWhenResponsePersistenceFails(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	agents := newTestAgentRepo()
	agents.agents["sender"] = domain.Agent{Name: "sender", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}

	tasks := newTestTaskRepo()
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "sender",
		Status:         domain.TaskStatusPending,
	}

	messages := newTestMessageRepo()
	messages.saveErr = errors.New("sqlite busy")

	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger(), projectRoot)

	_, err := svc.Send(context.Background(), SendResponseCmd{
		TaskID:  "t-001",
		Message: strings.Repeat("x", MaxInlineDeliveryChars+1),
	})
	if err == nil || !strings.Contains(err.Error(), "response was delivered but could not be persisted") {
		t.Fatalf("Send() error = %v, want persistence failure", err)
	}

	if len(panes.sent) != 1 {
		t.Fatalf("sent deliveries = %d, want 1", len(panes.sent))
	}
	if !strings.Contains(panes.sent[0].text, "payload_path=") {
		t.Fatalf("delivery text = %q, want payload_path notification", panes.sent[0].text)
	}
	if tasks.tasks["t-001"].Status != domain.TaskStatusPending {
		t.Fatalf("task status = %q, want pending", tasks.tasks["t-001"].Status)
	}
	if len(messages.responses) != 0 {
		t.Fatalf("stored responses = %d, want 0", len(messages.responses))
	}

	payloadDir := filepath.Join(projectRoot, ".myT-x", "orchestrator", "payloads")
	entries, readErr := os.ReadDir(payloadDir)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("ReadDir(%q): %v", payloadDir, readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("payload artifact count = %d, want 0", len(entries))
	}
}

func TestResponseServiceSendRollsBackStoredResponseWhenCompletionFails(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	agents := newTestAgentRepo()
	agents.agents["sender"] = domain.Agent{Name: "sender", PaneID: "%0"}
	agents.agents["worker"] = domain.Agent{Name: "worker", PaneID: "%1"}

	tasks := newTestTaskRepo()
	tasks.completeTaskErr = errors.New("sqlite busy")
	tasks.tasks["t-001"] = domain.Task{
		ID:             "t-001",
		AgentName:      "worker",
		AssigneePaneID: "%1",
		SenderName:     "sender",
		Status:         domain.TaskStatusPending,
	}

	messages := newTestMessageRepo()
	panes := &testPaneOps{selfPane: "%1"}
	svc := NewResponseService(agents, tasks, messages, panes, panes, discardLogger(), projectRoot)

	result, err := svc.Send(context.Background(), SendResponseCmd{
		TaskID:  "t-001",
		Message: strings.Repeat("x", MaxInlineDeliveryChars+1),
	})
	if err != nil {
		t.Fatalf("Send() error = %v, want warning result", err)
	}
	if !strings.Contains(result.Warning, "response persistence was rolled back") {
		t.Fatalf("warning = %q, want rollback warning", result.Warning)
	}
	if tasks.tasks["t-001"].Status != domain.TaskStatusPending {
		t.Fatalf("task status = %q, want pending", tasks.tasks["t-001"].Status)
	}
	if len(messages.responses) != 0 {
		t.Fatalf("stored responses = %d, want 0 after rollback", len(messages.responses))
	}

	payloadDir := filepath.Join(projectRoot, ".myT-x", "orchestrator", "payloads")
	entries, readErr := os.ReadDir(payloadDir)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("ReadDir(%q): %v", payloadDir, readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("payload artifact count = %d, want 0 after rollback", len(entries))
	}
}
