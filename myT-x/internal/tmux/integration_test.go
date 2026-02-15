package tmux

import (
	"runtime"
	"strings"
	"testing"

	"myT-x/internal/ipc"
)

func TestCommandRouterShimFlow(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only")
	}

	sessions := NewSessionManager()
	defer sessions.Close()

	router := NewCommandRouter(sessions, nil, RouterOptions{
		DefaultShell: "powershell.exe",
		PipeName:     `\\.\pipe\myT-x-test`,
		HostPID:      99999,
	})

	newResp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-d": true,
			"-s": "itest",
		},
	})
	if newResp.ExitCode != 0 {
		t.Fatalf("new-session failed: %s", newResp.Stderr)
	}

	hasResp := router.Execute(ipc.TmuxRequest{
		Command: "has-session",
		Flags: map[string]any{
			"-t": "itest",
		},
	})
	if hasResp.ExitCode != 0 {
		t.Fatalf("has-session failed: %s", hasResp.Stderr)
	}

	splitResp := router.Execute(ipc.TmuxRequest{
		Command: "split-window",
		Flags: map[string]any{
			"-h": true,
			"-t": "itest:0.0",
		},
		Env: map[string]string{
			"CLAUDE_CODE_AGENT_ID":   "researcher-1",
			"CLAUDE_CODE_AGENT_TYPE": "teammate",
			"CLAUDE_CODE_TEAM_NAME":  "itest",
		},
	})
	if splitResp.ExitCode != 0 {
		t.Fatalf("split-window failed: %s", splitResp.Stderr)
	}
	newPaneID := strings.TrimSpace(splitResp.Stdout)
	if newPaneID == "" || !strings.HasPrefix(newPaneID, "%") {
		t.Fatalf("split-window pane id invalid: %q", splitResp.Stdout)
	}

	listResp := router.Execute(ipc.TmuxRequest{
		Command: "list-panes",
		Flags: map[string]any{
			"-t": "itest:0",
		},
	})
	if listResp.ExitCode != 0 {
		t.Fatalf("list-panes failed: %s", listResp.Stderr)
	}
	if !strings.Contains(listResp.Stdout, "%0") || !strings.Contains(listResp.Stdout, "%1") {
		t.Fatalf("list-panes output missing pane ids: %q", listResp.Stdout)
	}

	displayResp := router.Execute(ipc.TmuxRequest{
		Command: "display-message",
		Flags: map[string]any{
			"-p": true,
			"-t": newPaneID,
		},
		Args: []string{"#{pane_id} #{session_name}"},
	})
	if displayResp.ExitCode != 0 {
		t.Fatalf("display-message failed: %s", displayResp.Stderr)
	}
	if !strings.Contains(displayResp.Stdout, "itest") {
		t.Fatalf("display-message output unexpected: %q", displayResp.Stdout)
	}

	killResp := router.Execute(ipc.TmuxRequest{
		Command: "kill-session",
		Flags: map[string]any{
			"-t": "itest",
		},
	})
	if killResp.ExitCode != 0 {
		t.Fatalf("kill-session failed: %s", killResp.Stderr)
	}
}
