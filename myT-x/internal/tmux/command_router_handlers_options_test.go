package tmux

import (
	"testing"

	"myT-x/internal/ipc"
)

func TestHandleSetOption(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		args  []string
	}{
		{
			name: "pane scope style update",
			flags: map[string]any{
				"-p": true,
				"-t": "%1",
			},
			args: []string{"pane-active-border-style", "bg=default,fg=colour33"},
		},
		{
			name: "format expansion flag is accepted as bool",
			flags: map[string]any{
				"-F": true,
				"-g": true,
			},
			args: []string{"status-left", "#{session_name}"},
		},
		{
			name: "empty request still succeeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(NewSessionManager(), &captureEmitter{}, RouterOptions{ShimAvailable: true})

			resp := router.Execute(ipc.TmuxRequest{
				Command: "set-option",
				Flags:   tt.flags,
				Args:    tt.args,
			})

			if resp.ExitCode != 0 {
				t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
			}
			if resp.Stdout != "" {
				t.Fatalf("Stdout = %q, want empty", resp.Stdout)
			}
		})
	}
}

func TestHandleSelectLayout(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		args  []string
	}{
		{
			name: "target and preset are ignored",
			flags: map[string]any{
				"-t": "demo:0",
			},
			args: []string{"main-vertical"},
		},
		{
			name: "empty request still succeeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(NewSessionManager(), &captureEmitter{}, RouterOptions{ShimAvailable: true})

			resp := router.Execute(ipc.TmuxRequest{
				Command: "select-layout",
				Flags:   tt.flags,
				Args:    tt.args,
			})

			if resp.ExitCode != 0 {
				t.Fatalf("ExitCode = %d, want 0, stderr=%q", resp.ExitCode, resp.Stderr)
			}
			if resp.Stdout != "" {
				t.Fatalf("Stdout = %q, want empty", resp.Stdout)
			}
		})
	}
}
