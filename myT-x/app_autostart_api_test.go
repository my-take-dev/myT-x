package main

import (
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

func TestStartAutoStartCommandValidation(t *testing.T) {
	t.Run("empty pane id", func(t *testing.T) {
		app := NewApp()
		if _, err := app.StartAutoStartCommand("   ", config.AutoStartCommand{Command: "codex"}); err == nil {
			t.Fatal("StartAutoStartCommand() expected pane id validation error")
		}
	})

	t.Run("empty command", func(t *testing.T) {
		app := NewApp()
		if _, err := app.StartAutoStartCommand("%1", config.AutoStartCommand{Command: "   "}); err == nil {
			t.Fatal("StartAutoStartCommand() expected command validation error")
		}
	})
}

func TestStartAutoStartCommandSplitsAndSendsRawCommandLine(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	_, pane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var calls []ipc.TmuxRequest
	app.sendKeys = sendKeysIO{
		executeRequest: func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			calls = append(calls, req)
			return ipc.TmuxResponse{ExitCode: 0}
		},
		sleep: func(time.Duration) {},
	}

	newPaneID, err := app.StartAutoStartCommand(pane.IDString(), config.AutoStartCommand{
		Name:    "Mini Codex",
		Command: "codex",
		Args:    "--model gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("StartAutoStartCommand() error = %v", err)
	}
	if newPaneID == "" || !app.sessions.HasPane(newPaneID) {
		t.Fatalf("new pane id = %q, pane should exist", newPaneID)
	}
	if len(calls) != 2 {
		t.Fatalf("send-keys calls = %d, want 2: %#v", len(calls), calls)
	}
	if calls[0].Command != "select-pane" || calls[0].Flags["-t"] != newPaneID {
		t.Fatalf("first request = %#v, want select-pane for new pane", calls[0])
	}
	if calls[1].Command != "send-keys" || calls[1].Flags["-t"] != newPaneID || calls[1].Flags["-N"] != true {
		t.Fatalf("second request = %#v, want CRLF send-keys for new pane", calls[1])
	}
	gotLine := strings.Join(calls[1].Args, " ")
	if gotLine != "codex --model gpt-5.4-mini Enter" {
		t.Fatalf("sent args = %q, want raw command line plus Enter", gotLine)
	}
}

func TestStartAutoStartCommandRollsBackPaneWhenSendFails(t *testing.T) {
	app := NewApp()
	app.sessions = tmux.NewSessionManager()
	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})

	session, pane, err := app.sessions.CreateSession("session-a", "0", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	var calls []ipc.TmuxRequest
	app.sendKeys = sendKeysIO{
		executeRequest: func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			calls = append(calls, req)
			if req.Command == "send-keys" {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: "write failed"}
			}
			return ipc.TmuxResponse{ExitCode: 0}
		},
		sleep: func(time.Duration) {},
	}

	newPaneID, err := app.StartAutoStartCommand(pane.IDString(), config.AutoStartCommand{
		Command: "codex",
	})
	if err == nil {
		t.Fatal("StartAutoStartCommand() expected send failure")
	}
	if newPaneID != "" {
		t.Fatalf("new pane id = %q, want empty on failure", newPaneID)
	}
	if len(calls) != 2 {
		t.Fatalf("send-keys calls = %d, want select-pane and send-keys: %#v", len(calls), calls)
	}
	if !app.sessions.HasPane(pane.IDString()) {
		t.Fatalf("original pane %s was removed during rollback", pane.IDString())
	}
	snapshot, ok := app.sessions.GetSession(session.Name)
	if !ok {
		t.Fatalf("session %q missing after rollback", session.Name)
	}
	if len(snapshot.Windows) != 1 || len(snapshot.Windows[0].Panes) != 1 {
		t.Fatalf("pane count after rollback = %#v, want one original pane", snapshot.Windows)
	}
	if snapshot.Windows[0].Panes[0].ID != pane.ID {
		t.Fatalf("remaining pane id = %s, want %s", snapshot.Windows[0].Panes[0].IDString(), pane.IDString())
	}
}
