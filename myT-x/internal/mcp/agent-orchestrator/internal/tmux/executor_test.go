package tmux

import (
	"context"
	"strings"
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func TestValidatePaneID(t *testing.T) {
	tests := []struct {
		name    string
		paneID  string
		wantErr bool
	}{
		{name: "valid single digit", paneID: "%1"},
		{name: "valid multiple digits", paneID: "%123"},
		{name: "empty", paneID: "", wantErr: true},
		{name: "missing percent", paneID: "1", wantErr: true},
		{name: "non digit suffix", paneID: "%1a", wantErr: true},
		{name: "negative", paneID: "%-1", wantErr: true},
		{name: "leading whitespace", paneID: " %1", wantErr: true},
		{name: "trailing newline", paneID: "%1\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePaneID(tt.paneID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePaneID(%q) error = %v, wantErr %v", tt.paneID, err, tt.wantErr)
			}
		})
	}
}

func TestExecutorMethodsRejectInvalidPaneID(t *testing.T) {
	exec := NewExecutor()
	ctx := context.Background()

	if err := exec.SendKeys(ctx, "invalid", "hello"); err == nil {
		t.Fatal("SendKeys should reject invalid pane_id")
	}
	if err := exec.SetPaneTitle(ctx, "invalid", "title"); err == nil {
		t.Fatal("SetPaneTitle should reject invalid pane_id")
	}
	if _, err := exec.CapturePaneOutput(ctx, "invalid", 10); err == nil {
		t.Fatal("CapturePaneOutput should reject invalid pane_id")
	}
}

func TestExecutorGetPaneIDUsesTMUXPANE(t *testing.T) {
	t.Setenv("TMUX_PANE", "%7")
	exec := NewExecutor()

	got, err := exec.GetPaneID(context.Background())
	if err != nil {
		t.Fatalf("GetPaneID: %v", err)
	}
	if got != "%7" {
		t.Fatalf("pane id = %q, want %%7", got)
	}
}

func TestExecutorGetPaneIDRejectsMissingTMUXPANE(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	exec := NewExecutor()

	_, err := exec.GetPaneID(context.Background())
	if err == nil {
		t.Fatal("GetPaneID should reject missing TMUX_PANE")
	}
}

func TestExecutorGetPaneIDRejectsInvalidTMUXPANE(t *testing.T) {
	t.Setenv("TMUX_PANE", "invalid")
	exec := NewExecutor()

	_, err := exec.GetPaneID(context.Background())
	if err == nil {
		t.Fatal("GetPaneID should reject invalid TMUX_PANE")
	}
}

func TestNewSessionAwareExecutor(t *testing.T) {
	exec := NewSessionAwareExecutor("my-session", false)
	if exec.SessionName != "my-session" {
		t.Fatalf("SessionName = %q, want %q", exec.SessionName, "my-session")
	}
	if exec.SessionAllPanes {
		t.Fatal("SessionAllPanes should be false")
	}

	execAll := NewSessionAwareExecutor("other", true)
	if !execAll.SessionAllPanes {
		t.Fatal("SessionAllPanes should be true")
	}
}

func TestNewExecutorDefaults(t *testing.T) {
	exec := NewExecutor()
	if exec.SessionName != "" {
		t.Fatalf("SessionName = %q, want empty", exec.SessionName)
	}
	if exec.SessionAllPanes {
		t.Fatal("SessionAllPanes should default to false")
	}
}

func TestFilterPanesBySession(t *testing.T) {
	panes := []domain.PaneInfo{
		{ID: "%1", Session: "session-a"},
		{ID: "%2", Session: "session-b"},
		{ID: "%3", Session: "session-a"},
		{ID: "%4", Session: "SESSION-A"}, // case insensitive match
	}

	filtered := filterPanesBySession(panes, "session-a")
	if len(filtered) != 3 {
		t.Fatalf("filtered count = %d, want 3", len(filtered))
	}
	for _, p := range filtered {
		if !strings.EqualFold(p.Session, "session-a") {
			t.Errorf("unexpected pane in filtered result: %+v", p)
		}
	}
}

func TestFilterPanesBySession_Empty(t *testing.T) {
	filtered := filterPanesBySession(nil, "session-a")
	if len(filtered) != 0 {
		t.Fatalf("filtered count = %d, want 0", len(filtered))
	}
}

func TestFilterPanesBySession_NoMatch(t *testing.T) {
	panes := []domain.PaneInfo{
		{ID: "%1", Session: "session-b"},
	}
	filtered := filterPanesBySession(panes, "session-a")
	if len(filtered) != 0 {
		t.Fatalf("filtered count = %d, want 0", len(filtered))
	}
}
