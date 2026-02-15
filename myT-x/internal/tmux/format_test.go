package tmux

import (
	"strings"
	"testing"
	"time"
)

func TestExpandFormatPaneVars(t *testing.T) {
	session := &TmuxSession{
		ID:        0,
		Name:      "claude-swarm",
		CreatedAt: time.Unix(1706745600, 0),
	}
	window := &TmuxWindow{
		ID:      0,
		Name:    "main",
		Session: session,
	}
	pane := &TmuxPane{
		ID:     3,
		Index:  1,
		Width:  120,
		Height: 30,
		Active: true,
		Window: window,
	}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := expandFormat("#{session_name} #{pane_id} #{pane_tty} #{pane_active}", pane)
	if !strings.Contains(got, "claude-swarm %3") {
		t.Fatalf("unexpected expanded format: %q", got)
	}
	if !strings.Contains(got, `\\.\conpty\%3`) {
		t.Fatalf("missing pane_tty: %q", got)
	}
	if !strings.HasSuffix(got, "1") {
		t.Fatalf("missing active flag: %q", got)
	}
}
