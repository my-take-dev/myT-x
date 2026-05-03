package tmux

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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
	if _, err := exec.SplitPane(ctx, "invalid", true); err == nil {
		t.Fatal("SplitPane should reject invalid pane_id")
	}
	if err := exec.SendKeysPaste(ctx, "invalid", "hello"); err == nil {
		t.Fatal("SendKeysPaste should reject invalid pane_id")
	}
}

func TestSendKeysWaitsAfterTextBeforeEnter(t *testing.T) {
	var events []string
	exec := newRecordingExecutor(t, &events, nil)

	if err := exec.SendKeys(context.Background(), "%1", "hello\n"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	want := []string{"select-pane", "send-keys", "sleep", "send-keys C-m"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysDoesNotSendEnterWhenPostTextWaitCanceled(t *testing.T) {
	var events []string
	ctx, cancel := context.WithCancel(context.Background())
	exec := newRecordingExecutor(t, &events, func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	})

	err := exec.SendKeys(ctx, "%1", "hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendKeys error = %v, want context.Canceled", err)
	}
	for _, event := range events {
		if event == "send-keys C-m" {
			t.Fatalf("send-keys C-m should not be sent after canceled wait: %v", events)
		}
	}
	want := []string{"select-pane", "send-keys", "sleep"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysChunkedDoesNotSendEnterWhenChunkWaitCanceled(t *testing.T) {
	var events []string
	sleepCalls := 0
	ctx, cancel := context.WithCancel(context.Background())
	exec := newRecordingExecutor(t, &events, func(ctx context.Context) error {
		sleepCalls++
		if sleepCalls == 2 {
			cancel()
			return ctx.Err()
		}
		return nil
	})

	err := exec.SendKeys(ctx, "%1", strings.Repeat("x", maxSendKeysLength+1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendKeys error = %v, want context.Canceled", err)
	}
	for _, event := range events {
		if event == "send-keys C-m" {
			t.Fatalf("send-keys C-m should not be sent after canceled chunk wait: %v", events)
		}
	}
	want := []string{"select-pane", "send-keys", "sleep", "send-keys", "sleep"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysPasteWaitsAfterPasteBeforeEnter(t *testing.T) {
	var events []string
	exec := newRecordingExecutor(t, &events, nil)

	if err := exec.SendKeysPaste(context.Background(), "%1", "hello\n"); err != nil {
		t.Fatalf("SendKeysPaste: %v", err)
	}

	want := []string{"select-pane", "load-buffer", "paste-buffer", "sleep", "send-keys C-m"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysPasteDoesNotSendEnterWhenPostPasteWaitCanceled(t *testing.T) {
	var events []string
	ctx, cancel := context.WithCancel(context.Background())
	exec := newRecordingExecutor(t, &events, func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	})

	err := exec.SendKeysPaste(ctx, "%1", "hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendKeysPaste error = %v, want context.Canceled", err)
	}
	for _, event := range events {
		if event == "send-keys C-m" {
			t.Fatalf("send-keys C-m should not be sent after canceled wait: %v", events)
		}
	}
	want := []string{"select-pane", "load-buffer", "paste-buffer", "sleep"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysPasteDoesNotDeleteBufferAfterSuccessfulPaste(t *testing.T) {
	var events []string
	deleteErr := errors.New("delete failed")
	exec := NewExecutor()
	exec.hooks = &executorHooks{
		combinedOutput: func(ctx context.Context, args ...string) ([]byte, error) {
			recordTmuxCommand(&events, args)
			if len(args) > 0 && args[0] == "delete-buffer" {
				return []byte("tmux delete refused"), deleteErr
			}
			return nil, nil
		},
		sleep: func(ctx context.Context, delay time.Duration) error {
			if delay != sendKeysDelay {
				t.Fatalf("sleep delay = %v, want %v", delay, sendKeysDelay)
			}
			events = append(events, "sleep")
			return nil
		},
	}

	err := exec.SendKeysPaste(context.Background(), "%1", "hello")
	if err != nil {
		t.Fatalf("SendKeysPaste error = %v, want nil", err)
	}
	want := []string{"select-pane", "load-buffer", "paste-buffer", "sleep", "send-keys C-m"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysPasteCleansUpAfterPasteFailure(t *testing.T) {
	var events []string
	pasteErr := errors.New("paste failed")
	exec := NewExecutor()
	exec.hooks = &executorHooks{
		combinedOutput: func(ctx context.Context, args ...string) ([]byte, error) {
			recordTmuxCommand(&events, args)
			if len(args) > 0 && args[0] == "paste-buffer" {
				return []byte("tmux paste refused"), pasteErr
			}
			return nil, nil
		},
	}

	err := exec.SendKeysPaste(context.Background(), "%1", "hello")
	if !errors.Is(err, pasteErr) {
		t.Fatalf("SendKeysPaste error = %v, want pasteErr", err)
	}
	if !strings.Contains(err.Error(), "paste-buffer") || !strings.Contains(err.Error(), "tmux paste refused") {
		t.Fatalf("SendKeysPaste error = %q, want paste context and tmux output", err.Error())
	}
	want := []string{"select-pane", "load-buffer", "paste-buffer", "delete-buffer"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSendKeysPasteJoinsCleanupFailureWithPasteFailure(t *testing.T) {
	var events []string
	pasteErr := errors.New("paste failed")
	deleteErr := errors.New("delete failed")
	exec := NewExecutor()
	exec.hooks = &executorHooks{
		combinedOutput: func(ctx context.Context, args ...string) ([]byte, error) {
			recordTmuxCommand(&events, args)
			if len(args) > 0 && args[0] == "paste-buffer" {
				return []byte("tmux paste refused"), pasteErr
			}
			if len(args) > 0 && args[0] == "delete-buffer" {
				return []byte("tmux delete refused"), deleteErr
			}
			return nil, nil
		},
	}

	err := exec.SendKeysPaste(context.Background(), "%1", "hello")
	if !errors.Is(err, pasteErr) {
		t.Fatalf("SendKeysPaste error = %v, want pasteErr", err)
	}
	if !errors.Is(err, deleteErr) {
		t.Fatalf("SendKeysPaste error = %v, want cleanup error wrapping deleteErr", err)
	}
	if !strings.Contains(err.Error(), "delete paste buffer") || !strings.Contains(err.Error(), "tmux delete refused") {
		t.Fatalf("SendKeysPaste error = %q, want cleanup context and tmux output", err.Error())
	}
	want := []string{"select-pane", "load-buffer", "paste-buffer", "delete-buffer"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
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

func recordTmuxCommand(events *[]string, args []string) {
	if len(args) == 0 {
		*events = append(*events, "")
		return
	}
	if args[0] == "send-keys" && args[len(args)-1] == "C-m" {
		*events = append(*events, "send-keys C-m")
		return
	}
	*events = append(*events, args[0])
}

func newRecordingExecutor(t *testing.T, events *[]string, onSleep func(context.Context) error) *RealExecutor {
	t.Helper()
	exec := NewExecutor()
	exec.hooks = &executorHooks{
		combinedOutput: func(ctx context.Context, args ...string) ([]byte, error) {
			recordTmuxCommand(events, args)
			return nil, nil
		},
		sleep: func(ctx context.Context, delay time.Duration) error {
			if delay != sendKeysDelay {
				t.Fatalf("sleep delay = %v, want %v", delay, sendKeysDelay)
			}
			*events = append(*events, "sleep")
			if onSleep != nil {
				return onSleep(ctx)
			}
			return nil
		},
	}
	return exec
}
