//go:build windows

package tmux

import (
	"context"
	"testing"
)

func TestNewTmuxCommandHidesWindow(t *testing.T) {
	cmd := newTmuxCommand(context.Background(), "display-message", "-p", "#{pane_id}")
	if cmd == nil {
		t.Fatal("newTmuxCommand() returned nil")
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("newTmuxCommand() left SysProcAttr nil")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("newTmuxCommand() did not enable HideWindow")
	}
}
