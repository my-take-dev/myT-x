package tmux

import (
	"context"
	"testing"
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
